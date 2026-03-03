# Build Image for MLflow Model Buildpack
FROM quay.io/fedora/fedora-minimal:43

# Use Yandex mirror (Russia) - replace metalink with baseurl
RUN sed -i 's|^metalink=.*|baseurl=https://mirror.yandex.ru/fedora/linux/releases/$releasever/Everything/$basearch/os/|g' /etc/yum.repos.d/fedora.repo && \
    sed -i 's|^metalink=.*|baseurl=https://mirror.yandex.ru/fedora/linux/updates/$releasever/Everything/$basearch/|g' /etc/yum.repos.d/fedora-updates.repo

# Install base dependencies
RUN dnf install -y \
    git-core \
    curl \
    tar \
    gzip \
    shadow-utils \
    && dnf clean all

# Install uv (fast Python package manager) to /usr/local/bin
RUN dnf install -y uv && \
    dnf clean all

# CNB user setup
ENV CNB_USER_ID=1000
ENV CNB_GROUP_ID=1000
RUN groupadd -g ${CNB_GROUP_ID} cnb && \
    useradd -u ${CNB_USER_ID} -g cnb -m -s /bin/bash cnb

# CNB environment
ENV CNB_STACK_ID="io.amazme.fedora-mlserver"
ENV CNB_BUILDPACKS_DIR="/cnb/buildpacks"
ENV CNB_LAYERS_DIR="/layers"
ENV CNB_APP_DIR="/workspace"

# Create CNB directories
RUN mkdir -p ${CNB_BUILDPACKS_DIR} ${CNB_LAYERS_DIR} ${CNB_APP_DIR} && \
    chown -R cnb:cnb /cnb /layers /workspace

# Set labels
LABEL io.buildpacks.stack.id="io.amazme.fedora-mlserver"

USER cnb
WORKDIR /workspace
