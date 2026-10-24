FROM node:22-alpine

WORKDIR /app
COPY server.js .

# home sits on the tmpfs the worker mounts at /tmp. the root fs is read-only.
ENV HOME=/tmp
ENV PORT=3000
USER node
EXPOSE 3000

CMD ["node", "server.js"]
