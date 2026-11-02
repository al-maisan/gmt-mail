# The gmt-mail container image. Build it with `make image`, which prepares the
# build context from the release archives (see scripts/image.sh); this file
# does not work on the source tree.
FROM scratch

ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION
ARG REVISION
ARG CREATED
ARG SOURCE

LABEL org.opencontainers.image.title="gmt-mail" \
	org.opencontainers.image.description="Send personalized emails in bulk using templates" \
	org.opencontainers.image.version="${VERSION}" \
	org.opencontainers.image.revision="${REVISION}" \
	org.opencontainers.image.created="${CREATED}" \
	org.opencontainers.image.source="${SOURCE}" \
	org.opencontainers.image.url="${SOURCE}" \
	org.opencontainers.image.documentation="${SOURCE}#container-image" \
	org.opencontainers.image.licenses="GPL-3.0-or-later"

# The modes come from the context; COPY --chmod would also set them on the
# parent directories with docker.
# /etc/ssl/certs/ca-certificates.crt: the SMTP server's certificate is checked.
COPY rootfs/ /
# /gmt-mail
COPY ${TARGETARCH}${TARGETVARIANT}/ /

USER 65532:65532
# Mount the directory with the config, the template, the attachments and .env
# here; gmt-mail loads .env from it.
WORKDIR /work
ENTRYPOINT ["/gmt-mail"]
