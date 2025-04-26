FROM mcr.microsoft.com/devcontainers/go:1-bullseye

# Install additional Linux packages
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends \
    zsh \
    curl \
    wget \
    git \
    && apt-get clean -y \
    && rm -rf /var/lib/apt/lists/*

# Install additional Go tools
RUN go install -v golang.org/x/tools/gopls@latest
RUN go install -v github.com/go-delve/delve/cmd/dlv@latest
RUN go install -v honnef.co/go/tools/cmd/staticcheck@latest

# Set up non-root user
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

# [Optional] Set the default user
USER $USERNAME
