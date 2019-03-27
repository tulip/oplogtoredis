FROM tulip/meteor-build-env:1.6.1.1_0

ADD . /src
WORKDIR /src

RUN meteor npm install && \
  meteor build --directory /app && \
  cd /app/bundle/programs/server && \
  npm install


FROM node:8.9.4-slim

# Install mongoDB client
RUN apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv 2930ADAE8CAF5059EE73BB4B58712A2291FA4AD5
RUN echo "deb http://repo.mongodb.org/apt/debian jessie/mongodb-org/3.6 main" > /etc/apt/sources.list.d/mongodb-org-3.6.list

RUN apt-get update && apt-get install -y mongodb-org-shell=3.6.11 netcat

COPY --from=0 /app/bundle /app
WORKDIR /app
CMD node main.js

ENV PORT 8080
EXPOSE 8080
