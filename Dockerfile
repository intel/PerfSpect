FROM ubuntu:22.04@sha256:0e5e4a57c2499249aafc3b40fcd541e9a456aab7296681a3994d631587203f97

# Set environment variables for proxy, locale, and non-interactive installation
ENV http_proxy=${http_proxy} \
    https_proxy=${https_proxy} \
    DEBIAN_FRONTEND=noninteractive \
    LANG=en_US.UTF-8 \
    LC_ALL=en_US.UTF-8

# Install locales, set UTF-8 encoding, install dependencies, cleanup
RUN apt-get update --fix-missing \
    && apt-get install -y locales \
    && sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen \
    && locale-gen en_US.UTF-8 \
    && dpkg-reconfigure --frontend=noninteractive locales \
    && update-locale LANG=en_US.UTF-8 \
    && apt-get install -y sudo \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Copy the perfspect binary to the container
COPY ./perfspect /usr/bin/perfspect
RUN mkdir -p /output
WORKDIR /output
# ENTRYPOINT ["perfspect"]
# CMD ["-h"]