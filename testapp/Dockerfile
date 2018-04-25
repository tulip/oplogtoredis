FROM ubuntu:16.04

ENV METEOR_ALLOW_SUPERUSER=true

RUN mkdir /src
WORKDIR /src

RUN apt-get update
RUN apt-get install -y g++ build-essential curl
RUN curl https://install.meteor.com/ | sh

CMD meteor run --port 0.0.0.0:3000 --settings settings.json
