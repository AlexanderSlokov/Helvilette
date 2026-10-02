# Helvilette node images for the e2e stack: an Ubuntu 24.04 userland with systemd
# as PID 1, so Othela and the Agent run as real units and journald is a real
# service. Foreground-process containers cannot exercise either, and Helvilette
# exists to manage systemd. See ADR-0007 R2.
#
# Four stages in one file so the Go builder and the shared base are written once.
# Compose selects othela-node or agent-node with `target:`. A separately built
# base image would need building before Compose runs, which Compose has no way
# to order.
#
# Build context is the repository root.

FROM golang:1.25.6 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO_ENABLED=1 for mattn/go-sqlite3, the same driver k3s/kine uses. Building on
# the golang image's older glibc and running on Ubuntu 24.04's newer one is the
# safe direction; the reverse fails at load time.
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/othela ./cmd/othela \
 && CGO_ENABLED=0 GOOS=linux go build -o /out/agent ./cmd/agent


# The image Molecule uses to test Ansible roles: systemd as PID 1, Ansible
# preinstalled, udev and getty units already stripped.
FROM geerlingguy/docker-ubuntu2404-ansible:latest AS node-base

# SIGRTMIN+3 is systemd's shutdown signal. Without this, `docker stop` sends
# SIGTERM, which systemd as PID 1 interprets as "reexec", and the container
# sits until the kill timeout expires.
STOPSIGNAL SIGRTMIN+3

# journald keeps its log under /var/log/journal when the directory exists, and
# falls back to volatile storage in /run otherwise. The directory is created
# here so `journalctl` inside the container returns something after a restart,
# which is what makes ADR-0007 D2 testable.
RUN mkdir -p /var/log/journal \
 && systemd-tmpfiles --create --prefix /var/log/journal || true

# curl is the health probe and the first thing anyone reaches for when debugging
# inside a node. The base image ships libcurl but not the binary.
RUN apt-get update \
 && apt-get install -y --no-install-recommends curl \
 && rm -rf /var/lib/apt/lists/*

CMD ["/lib/systemd/systemd"]


FROM node-base AS othela-node

# The unit runs as this user, and StateDirectory= chowns /var/lib/helvilette/othela
# to it on every start. A system account with no shell: Othela never needs a login.
RUN useradd --system --no-create-home --shell /usr/sbin/nologin helvilette

COPY --from=builder /out/othela /usr/local/bin/othela
COPY tests/images/node/helvilette-othela.service /etc/systemd/system/

# Offline enable: systemctl writes the WantedBy symlink without a running
# systemd, so the unit is active on first boot with no entrypoint script.
RUN systemctl enable helvilette-othela.service

EXPOSE 8080


FROM node-base AS agent-node

COPY --from=builder /out/agent /usr/local/bin/agent
COPY tests/images/node/helvilette-agent.service /etc/systemd/system/

RUN systemctl enable helvilette-agent.service
