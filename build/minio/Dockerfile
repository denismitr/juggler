FROM minio/mc

COPY ./build/minio/entrypoint.sh /etc/entrypoint.sh

RUN chmod +x /etc/entrypoint.sh

ENTRYPOINT ["/etc/entrypoint.sh"]
