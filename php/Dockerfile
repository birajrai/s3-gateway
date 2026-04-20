FROM php:8.2-apache

RUN apt-get update && apt-get install -y \
    libxml2-dev \
    && docker-php-ext-install xml \
    && a2enmod rewrite

RUN mkdir -p /var/www/html /var/www/html/data

COPY .htaccess /var/www/html/
COPY index.php /var/www/html/
COPY src/ /var/www/html/src/
COPY .config.ini /var/www/html/

RUN chown -R www-data:www-data /var/www/html

EXPOSE 80

ENV DATA_DIR=/var/www/html/data

CMD ["apache2-foreground"]