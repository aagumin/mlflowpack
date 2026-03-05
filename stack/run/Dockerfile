# Run Image for MLflow Model Buildpack
# Buildpack API 0.12 compatible
FROM quay.io/fedora/fedora-minimal:43

# Use Yandex mirror (Russia) - replace metalink with baseurl
RUN sed -i 's|^metalink=.*|baseurl=https://mirror.yandex.ru/fedora/linux/releases/$releasever/Everything/$basearch/os/|g' /etc/yum.repos.d/fedora.repo && \
    sed -i 's|^metalink=.*|baseurl=https://mirror.yandex.ru/fedora/linux/updates/$releasever/Everything/$basearch/|g' /etc/yum.repos.d/fedora-updates.repo

# Install Python 3.11 and dependencies
RUN dnf install -y \
    python3.11 \
    shadow-utils \
    && dnf clean all

# Setup pip for python3.11
RUN python3.11 -m ensurepip --upgrade && \
    alternatives --install /usr/bin/python3 python3 /usr/bin/python3.11 1 && \
    alternatives --install /usr/bin/pip3 pip3 /usr/local/bin/pip3.11 1

# Install MLServer base + mlflow runtime only
# Other runtimes (sklearn, xgboost, lightgbm) will be installed by buildpack based on model type
RUN pip3 install --no-cache-dir --break-system-packages \
    mlserver==1.7.1 \
    mlserver-mlflow

# CNB user setup (required for buildpack API 0.12)
ARG cnb_uid=1000
ARG cnb_gid=1000
ENV CNB_USER_ID=${cnb_uid}
ENV CNB_GROUP_ID=${cnb_gid}

# Create user and group
RUN groupadd cnb --gid ${CNB_GROUP_ID} && \
    useradd --uid ${CNB_USER_ID} --gid ${CNB_GROUP_ID} -m -s /bin/bash cnb

# Set required CNB target labels (API 0.12)
LABEL io.buildpacks.base.distro.name="fedora"
LABEL io.buildpacks.base.distro.version="43"

# MLServer defaults
ENV MLSERVER_MODEL_NAME="model"
ENV MLSERVER_MODEL_URI="/opt/mlflow/model"
ENV MLSERVER_MODEL_IMPLEMENTATION="mlserver_mlflow.MLflowRuntime"
ENV MLSERVER_HOST="0.0.0.0"
ENV MLSERVER_HTTP_PORT="8080"
ENV MLSERVER_GRPC_PORT="9080"

# Create model directory
RUN mkdir -p /opt/mlflow && chown -R cnb:cnb /opt/mlflow

USER ${CNB_USER_ID}:${CNB_GROUP_ID}
WORKDIR /home/cnb

EXPOSE 8080 9080

# Default entrypoint - can be overridden by the buildpack
ENTRYPOINT ["mlserver", "start", "/opt/mlflow/model"]
