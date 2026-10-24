FROM alpine:3.9

# Setting Env
ENV LANG="zh_TW.UTF8"
ENV LC_ALL="zh_TW.UTF8"

RUN mkdir -p /app
WORKDIR /app

ARG VERSION

ENV VERSION ${VERSION}

ADD bin/target-hall /app/target-hall
ADD views /app/views
# ADD static /app/static
ADD static2 /app/static2
ADD i18n /app/i18n

CMD [ "/app/target-hall" ]