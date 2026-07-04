# syntax=docker/dockerfile:1
#
# Build context is the repository root (see deploy/docker-compose.yml) so
# this stage can also pick up deploy/nginx.conf without reaching outside the
# build context.
FROM node:22-alpine AS build
WORKDIR /src

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ .
RUN npm run build

FROM nginx:1.27-alpine AS runtime
COPY --from=build /src/dist /usr/share/nginx/html
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
