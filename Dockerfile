FROM gcr.io/distroless/static-debian12:nonroot
COPY llm-router /llm-router
EXPOSE 8080
ENTRYPOINT ["/llm-router", "serve"]