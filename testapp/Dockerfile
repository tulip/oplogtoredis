FROM tulip/meteor-build-env:1.6.1.1_0

ADD . /src
WORKDIR /src

RUN meteor npm install && \
  meteor build --directory /app && \
  cd /app/bundle/programs/server && \
  npm install


FROM node:8.9.4-slim

COPY --from=0 /app/bundle /app
WORKDIR /app
CMD node main.js

ENV PORT 8080
EXPOSE 8080
