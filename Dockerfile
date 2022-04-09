# https://goreleaser.com/customization/docker
FROM scratch
COPY fonzie /
ENTRYPOINT ["/fonzie"]
