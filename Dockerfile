# Use a lightweight web server image
FROM nginx:alpine

# Copy all files from the current directory into nginx's web root
COPY . /usr/share/nginx/html

# Expose port 8080
EXPOSE 8080

# Configure nginx to listen on port 8080
RUN sed -i 's/listen       80;/listen       8080;/g' /etc/nginx/conf.d/default.conf

# Start nginx
CMD ["nginx", "-g", "daemon off;"]