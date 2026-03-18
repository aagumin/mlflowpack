# Run Image for MLflow Model Buildpack
# Buildpack API 0.12 compatible
FROM quay.io/fedora/fedora-minimal:43

# The run image stays intentionally thin: Python, MLServer, and model
# dependencies are provided by buildpack launch layers.

# CNB user setup (required for buildpack API 0.12)
ARG cnb_uid=1000
ARG cnb_gid=1000
ENV CNB_USER_ID=${cnb_uid}
ENV CNB_GROUP_ID=${cnb_gid}

# Set required CNB target labels (API 0.12)
LABEL io.buildpacks.base.distro.name="fedora"
LABEL io.buildpacks.base.distro.version="43"

USER ${CNB_USER_ID}:${CNB_GROUP_ID}
WORKDIR /

EXPOSE 8080 9080
