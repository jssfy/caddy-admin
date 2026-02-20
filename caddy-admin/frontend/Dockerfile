FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json .
RUN npm install
COPY . .
RUN npm run build

FROM alpine:3.19
# Frontend static files served by Caddy directly; this stage just exports dist/
COPY --from=builder /app/dist /dist
