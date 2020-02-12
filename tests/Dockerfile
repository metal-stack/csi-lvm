FROM alpine

ENV KUBECONFIG /files/.kubeconfig

RUN apk add --update ca-certificates \
 && apk add --update -t deps curl bats \
 && curl -L https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl  -o /usr/local/bin/kubectl \
 && chmod +x /usr/local/bin/kubectl

COPY bats /bats
COPY files /files

