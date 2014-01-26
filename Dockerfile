# Freestore
#
# Version 1

# Base on the Fedora image created by Matthew
FROM mattdm/fedora

MAINTAINER Mateus Braga <mateus.a.braga@gmail.com>

# Update packages
RUN yum update -y

# Install dependencies
RUN yum install -y gcc golang git

#set GOPATH
ENV GOPATH /go


# install freestore server
RUN go get github.com/mateusbraga/freestore/...

# By default, launch freestore server on port 5000
CMD ["/go/bin/freestored", "-bind :5000"]

#expose freestore port
EXPOSE 5000

