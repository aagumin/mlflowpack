# Build Image for MLflow Model Buildpack
# Buildpack API 0.12 compatible
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

# Install uv (fast Python package manager)
RUN dnf install -y uv && \
    dnf clean all

# CNB user setup (required for buildpack API 0.12)
ARG cnb_uid=1001
ARG cnb_gid=1001
ENV CNB_USER_ID=${cnb_uid}
ENV CNB_GROUP_ID=${cnb_gid}

# Create user and group
RUN groupadd cnb --gid ${CNB_GROUP_ID} && \
    useradd --uid ${CNB_USER_ID} --gid ${CNB_GROUP_ID} -m -s /bin/bash cnb

# Set required CNB target labels (API 0.12)
LABEL io.buildpacks.base.distro.name="fedora"
LABEL io.buildpacks.base.distro.version="43"

USER ${CNB_USER_ID}:${CNB_GROUP_ID}
WORKDIR /workspace
