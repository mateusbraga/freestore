# Freestore
#
# Version 1

# Base on the Fedora image created by Matthew
FROM mattdm/fedora

MAINTAINER Mateus Braga <mateus.a.braga@gmail.com>

# Update packages
#RUN yum update -y

# Install gcc
RUN yum install gcc -y
# Install golang
RUN yum install golang -y
# Install git
RUN yum install git -y

#set GOPATH
ENV GOPATH /go


# install freestore server
RUN go get github.com/mateusbraga/freestore/cmd/freestored

# Launch freestore server on port 5000
ENTRYPOINT ["/go/bin/freestored"]

CMD ["-p 5000"]

# run the server as the daemon user
USER freestored

#expose freestore port
EXPOSE 5000

