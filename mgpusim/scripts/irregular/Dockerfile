FROM ubuntu:24.04@sha256:66460d557b25769b102175144d538d88219c077c678a49af4afca6fbfc1b5252
# LLVM stays isolated from the host. Set APT_CHECK_DATE=false only for mirrors
# with future Release dates relative to the simulation machine's clock.
ARG APT_CHECK_DATE=true
RUN apt-get -o Acquire::Check-Date=$APT_CHECK_DATE update -qq && \
    apt-get -o Acquire::Check-Date=$APT_CHECK_DATE install -y --no-install-recommends clang-14=1:14.0.6-19build4 lld-14=1:14.0.6-19build4 && \
    rm -rf /var/lib/apt/lists/*
