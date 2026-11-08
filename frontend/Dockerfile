# syntax=docker/dockerfile:1

# ---- build the static bundle ----
FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
# Shows the demo-credentials hint on the login page. Leave false for anything but a local demo.
ARG VITE_DEMO_HINT=false
ENV VITE_DEMO_HINT=$VITE_DEMO_HINT
RUN npm run build

# ---- serve with nginx (unprivileged image: runs as non-root on port 8080) ----
FROM nginxinc/nginx-unprivileged:stable-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 8080
