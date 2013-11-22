# Freestore
#
# Version 1

# Base on the Fedora image created by Matthew
FROM mattdm/fedora

MAINTAINER Mateus Braga <mateus.a.braga@gmail.com>

# Update packages
#RUN yum update -y

# Install golang
RUN yum install golang -y

# install freestore server
RUN go install github.com/mateusbraga/freestore

# Launch freestore server on port 5000
ENTRYPOINT ["server"]

CMD ["-p 5000"]

# run the server as the daemon user
USER daemon

#expose freestore port
EXPOSE 5000

