# Freestore

Freestore is a library that implements a consistent and reconfigurable distributed memory. 

Freestore implements a storage abstraction with READ/WRITE operations. It replicates the data over all servers and uses the idea that two majorities always intersect to provide fault-tolerance and strong consistency (strict quorum). Freestore also implements a fault-tolerant reconfiguration mechanism (other than consensus protocol - no agreement is required) that allows the cluster to continue operating normally during configuration changes.

This Freestore implementation is the result of Mateus Braga's [Bachelor's thesis (Portuguese)][thesis] at University of Brasilia, Brazil. Eduardo Alchieri's Freestore original paper in Portuguese is here: [http://sbrc2014.ufsc.br/anais/files/trilha/ST07-2.pdf][freestore-article].

[![GoDoc](https://godoc.org/github.com/mateusbraga/freestore?status.png)](https://godoc.org/github.com/mateusbraga/freestore)
[![Build Status](https://travis-ci.org/mateusbraga/freestore.png?branch=master)](https://travis-ci.org/mateusbraga/freestore)

[thesis]: http://www.mateusbraga.com.br/files/Monografia%20Mateus%20Antunes%20Braga.pdf
[freestore-article]: http://sbrc2014.ufsc.br/anais/files/trilha/ST07-2.pdf

# Fault Tolerance Model

[Freestore's Fault Tolerance Model](https://github.com/mateusbraga/freestore/blob/master/docs/fault-tolerance-model.md)

# Development

Freestore is written in [Go](http://golang.org). You'll need a recent version of Go installed on your computer to build Freestore.

## Build

go build ./...

## Test

go test ./...

# Running

    freestore_server -bind :5000
    freestore_server -bind :5001
    freestore_server -bind :5002

    freestore_client

