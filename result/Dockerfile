FROM node:24.22.3-trixie-slim

ENV NODE_ENV=production

WORKDIR /app

COPY package*.json ./

RUN npm ci --only=production && npm cache clean --force

COPY --chown=node:node . .

USER node

EXPOSE 8888

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD node -e "require('http').get('http://localhost:8888', (res) => process.exit(res.statusCode === 200 ? 0 : 1))"

CMD ["node", "server.js"]