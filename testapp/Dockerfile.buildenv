# This construct a build environment for Meteor, with standard packages
# pre-cached. It should be pushed as tulip/meteor-build-env:<meteor version>_<revision>,
# e.g. tulip/meteor-build-env:1.6.1_0 (for the first revision of the build env
# for 1.6.1)

FROM node:8.15.1-jessie

ENV METEOR_ALLOW_SUPERUSER=true

RUN apt-get update && \
  apt-get install -y g++ build-essential curl && \
  curl https://install.meteor.com/ | sh

RUN meteor create --release 1.6.1.1 /throwaway && rm -rf /throwaway
