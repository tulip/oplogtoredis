FROM --platform=linux/amd64 meteor/node:14.21.4

# Install Meteor
ENV METEOR_ALLOW_SUPERUSER=true

RUN apt-get update && \
  apt-get install -y g++ build-essential curl && \
  rm -rf /var/lib/apt/lists/* && \
  curl https://install.meteor.com/?release=2.16 | sh

RUN meteor create --release 2.16 /throwaway && rm -rf /throwaway

# Set up app
ADD . /src
WORKDIR /src

RUN meteor npm install && \
  meteor build --directory /app && \
  cd /app/bundle/programs/server && \
  npm install

FROM --platform=linux/amd64 meteor/node:14.21.4

# Install mongoDB client
RUN apt-get update && apt-get install -y gnupg wget

RUN wget -qO - https://pgp.mongodb.com/server-7.0.asc | gpg -o /usr/share/keyrings/mongodb-server-7.0.gpg --dearmor
RUN echo "deb [ signed-by=/usr/share/keyrings/mongodb-server-7.0.gpg ] https://repo.mongodb.org/apt/ubuntu focal/mongodb-org/7.0 multiverse" > /etc/apt/sources.list.d/mongodb-org-7.0.list

RUN apt-get update && apt-get install -y mongodb-org-shell mongodb-mongosh netcat-openbsd && rm -rf /var/lib/apt/lists/*

COPY --from=0 /app/bundle /app
WORKDIR /app
CMD node main.js

ENV PORT 8080
EXPOSE 8080
