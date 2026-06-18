FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json* ./

RUN npm install --legacy-peer-deps

COPY . .

ENV NUXT_PUBLIC_API_BASE=/api/v1

RUN npm run build

RUN ls -la .output/public/ || echo "Build output check"

FROM nginx:alpine

RUN rm -f /usr/share/nginx/html/index.html /usr/share/nginx/html/50x.html

COPY --from=builder /app/.output/public /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

RUN ls -la /usr/share/nginx/html/ || echo "Nginx html dir check"

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
