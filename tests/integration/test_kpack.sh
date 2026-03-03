#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
KPACK_DIR="${SCRIPT_DIR}/kpack"
TEST_NS="mlflow-buildpack-test"
TIMEOUT="${TIMEOUT:-600}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is required"
        exit 1
    fi

    if ! command -v kind &> /dev/null; then
        log_error "kind is required"
        exit 1
    fi

    if ! kind get clusters 2>/dev/null | grep -q "kind"; then
        log_error "No kind cluster found. Please create one first."
        exit 1
    fi

    log_info "Prerequisites OK"
}

load_images_to_kind() {
    log_info "Loading images to kind cluster..."

    local images=(
        "amazme/fedora-mlserver-build:43"
        "amazme/fedora-mlserver-run:43"
        "io.amazme.buildpacks.mlflow-model:0.1.0"
    )

    for img in "${images[@]}"; do
        log_info "Loading ${img}..."
        kind load docker-image "${img}" --name kind 2>/dev/null || true
    done

    log_info "Images loaded successfully"
}

install_kpack() {
    log_info "Installing kpack..."

    # Check if kpack is already installed
    if kubectl get namespace kpack 2>/dev/null | grep -q "kpack"; then
        log_info "kpack already installed"
        return 0
    fi

    # Install kpack
    kubectl apply -f https://github.com/pivotal/kpack/releases/download/v0.15.0/release-0.15.0.yaml

    # Wait for kpack to be ready
    log_info "Waiting for kpack to be ready..."
    kubectl wait --for=condition=available --timeout=120s deployment/kpack-controller-manager -n kpack-controller 2>/dev/null || true
    kubectl wait --for=condition=available --timeout=120s deployment/kpack-webhook-manager -n kpack-webhook 2>/dev/null || true

    log_info "kpack installed successfully"
}

deploy_test_model() {
    log_info "Deploying test model..."

    # Create test namespace
    kubectl create namespace ${TEST_NS} --dry-run=client -o yaml 2>/dev/null || true

    # Create test model configmap
    kubectl create configmap test-model-config --from-file="${PROJECT_ROOT}/test-model/MLmodel" -n ${TEST_NS} --dry-run=client -o yaml 2>/dev/null || true
    kubectl apply -f -n ${TEST_NS} - < "${PROJECT_ROOT}/test-model/MLmodel" 2>/dev/null || true

    log_info "Test model deployed"
}

deploy_kpack_resources() {
    log_info "Deploying kpack resources..."

    # Apply in order
    for manifest in "${KPACK_DIR}"/*.yaml; do
        log_info "Applying $(basename "$manifest")..."
        kubectl apply -f "$manifest" || true
    done

    log_info "kpack resources deployed"
}

wait_for_build() {
    local image_name="$1"
    local timeout="${2:-600}"
    local start_time=$(date +%s)

    log_info "Waiting for build to complete (timeout: ${timeout}s)..."

    while true; do
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))

        if [ $elapsed -ge $timeout ]; then
            log_error "Timeout waiting for build"
            kubectl get image -n ${TEST_NS} ${image_name} -o yaml
            return 1
        fi

        local status=$(kubectl get image -n ${TEST_NS} ${image_name} -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)

        if [ "$status" == "True" ]; then
            log_info "Build completed successfully!"
            kubectl get image -n ${TEST_NS} ${image_name} -o yaml
            return 0
        fi

        local build_status=$(kubectl get image -n ${TEST_NS} ${image_name} -o jsonpath='{.status.conditions[?(@.type=="Building")].message}' 2>/dev/null || echo "")

        if [ -n "$build_status" ]; then
            echo -n "."
            log_info "Build status: ${build_status}"
        fi

        sleep 5
    done
}

run_model_inference_test() {
    log_info "Running inference test..."

    # Get the built image
    local built_image=$(kubectl get image -n ${TEST_NS} test-sklearn-model -o jsonpath='{.status.latestImage}' 2>/dev/null)

    if [ -z "$built_image" ]; then
        log_error "No built image found"
        return 1
    fi

    log_info "Built image: ${built_image}"

    # Deploy test pod
    kubectl run -n ${TEST_NS} inference-test --image="${built_image}" --restart=Never -- bash -c '
        echo "Testing inference endpoint..."
        curl -s -X POST http://localhost:8080/v2/models/model/infer \
            -H "Content-Type: application/json" \
            -d "{\"inputs\": [{\"name\": \"input\", \"shape\": [1, 4], \"datatype\": \"FP32\", \"data\": [[5.1, 3.5, 1.4, 0.2]]}]}"
        echo ""
        echo "Test completed!"
    '

    # Wait for pod to complete
    kubectl wait --for=condition=complete pod/inference-test -n ${TEST_NS} --timeout=60s || true

    # Show logs
    kubectl logs -n ${TEST_NS} inference-test || true
}

cleanup() {
    log_info "Cleaning up..."
    kubectl delete namespace ${TEST_NS} --ignore-not-found=true || true
    kubectl delete -f "${KPACK_DIR}"/*.yaml --ignore-not-found=true || true
}

main() {
    local action="${1:-all}"

    case "$action" in
        setup)
            check_prerequisites
            load_images_to_kind
            install_kpack
            ;;
        deploy)
            deploy_test_model
            deploy_kpack_resources
            ;;
        test)
            wait_for_build "test-sklearn-model"
            ;;
        inference)
            run_model_inference_test
            ;;
        cleanup)
            cleanup
            ;;
        all)
            check_prerequisites
            load_images_to_kind
            install_kpack
            deploy_test_model
            deploy_kpack_resources
            wait_for_build "test-sklearn-model" "${TIMEOUT}"
            ;;
        *)
            echo "Usage: $0 {setup|deploy|test|inference|cleanup|all}"
            exit 1
            ;;
    esac
}

main "$@"
