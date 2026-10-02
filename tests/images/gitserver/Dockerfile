# Git daemon serving the e2e playbook and fleet repositories over git://.
#
# The repositories are committed into the image at build time. Nothing seeds them
# at container start, and nothing is bind-mounted from the source tree, so no
# container ever writes into the repository working directory. See ADR-0007 D4.
#
# Build context is the repository root.
FROM alpine:3.19

# Installed at build time, not at container start. An inline `apk add` made every
# e2e run depend on a package download finishing inside the startup deadline: it
# failed under load and passed on an idle host. See ADR-0003.
RUN apk add --no-cache git git-daemon

COPY tests/images/gitserver/bake-repos.sh /usr/local/bin/bake-repos.sh
COPY tests/images/gitserver/seed-commit.sh /usr/local/bin/seed-commit.sh
RUN chmod 0755 /usr/local/bin/bake-repos.sh /usr/local/bin/seed-commit.sh

COPY tests/fixtures/ /src/
RUN /usr/local/bin/bake-repos.sh /src /git && rm -rf /src

EXPOSE 9418

# git ls-remote over the loopback is the cheapest true statement about
# readiness: the daemon is accepting connections and can serve a ref.
HEALTHCHECK --interval=2s --timeout=5s --start-period=2s --retries=15 \
    CMD git ls-remote git://localhost/fleet HEAD >/dev/null 2>&1

# --export-all serves every repository under the base path without a
# git-daemon-export-ok marker in each. Acceptable because this image exists only
# to serve disposable fixtures.
CMD ["git", "daemon", \
     "--verbose", \
     "--export-all", \
     "--base-path=/git", \
     "--reuseaddr", \
     "--enable=receive-pack"]
