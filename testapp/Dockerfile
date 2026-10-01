FROM --platform=linux/amd64 node:14.21.3-bullseye
# Debian no longer serves all of bullseye-security, so it comes from Debian's snapshot archive.
RUN sed -i -E 's#http://(deb|security)\.debian\.org/debian-security#http://snapshot.debian.org/archive/debian-security/20260824T000000Z#' /etc/apt/sources.list && \
  echo 'Acquire::Check-Valid-Until "false";' > /etc/apt/apt.conf.d/99snapshot

# Install Meteor
ENV METEOR_ALLOW_SUPERUSER=true

RUN apt-get update && \
  apt-get install -y g++ build-essential curl && \
  rm -rf /var/lib/apt/lists/* && \
  curl https://install.meteor.com/?release=2.13 | sh

RUN meteor create --release 2.5.6 /throwaway && rm -rf /throwaway

# Set up app
ADD . /src
WORKDIR /src

RUN meteor npm install && \
  meteor build --directory /app && \
  cd /app/bundle/programs/server && \
  npm install

FROM --platform=linux/amd64 node:14.21.3-bullseye-slim
# Debian no longer serves all of bullseye-security, so it comes from Debian's snapshot archive.
RUN sed -i -E 's#http://(deb|security)\.debian\.org/debian-security#http://snapshot.debian.org/archive/debian-security/20260824T000000Z#' /etc/apt/sources.list && \
  echo 'Acquire::Check-Valid-Until "false";' > /etc/apt/apt.conf.d/99snapshot

# Install mongoDB client
RUN apt-get update && apt-get install -y gnupg wget

RUN wget -qO - https://pgp.mongodb.com/server-5.0.asc | gpg -o /usr/share/keyrings/mongodb-server-5.0.gpg --dearmor
RUN echo "deb [ signed-by=/usr/share/keyrings/mongodb-server-5.0.gpg ] https://repo.mongodb.org/apt/debian bullseye/mongodb-org/5.0 main" > /etc/apt/sources.list.d/mongodb-org-5.0.list

RUN apt-get update && apt-get install -y mongodb-org-shell netcat && rm -rf /var/lib/apt/lists/*

COPY --from=0 /app/bundle /app
WORKDIR /app
CMD node main.js

ENV PORT 8080
EXPOSE 8080
