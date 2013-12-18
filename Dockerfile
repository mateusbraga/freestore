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
ENV GOPATH /opt


# install freestore server
RUN go get github.com/mateusbraga/freestore/samples/server

# Launch freestore server on port 5000
ENTRYPOINT ["/opt/bin/server"]

CMD ["-p 5000"]

# run the server as the daemon user
USER daemon

#expose freestore port
EXPOSE 5000

