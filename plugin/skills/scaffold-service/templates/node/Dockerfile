FROM node:20-alpine
WORKDIR /app
COPY package.json .
RUN npm install --omit=dev
COPY index.js .
EXPOSE __BACKEND_PORT__
CMD ["node", "index.js"]
