# Freestore

Freestore is a research project developed as part of Mateus Braga's [Bachelor's thesis (Portuguese)][thesis] at University of Brasilia, Brazil. It is a library that implements a fault-tolerant, consistent, and reconfigurable distributed memory. 

Freestore implements a storage abstraction with READ/WRITE operations. It replicates the data over all servers and uses the idea that two majorities always intersect to provide fault-tolerance and strong consistency (strict quorum - read most recently written value). Freestore also implements a fault-tolerant reconfiguration mechanism (other than consensus protocol - no agreement is required) that allows the cluster to continue operating normally during configuration changes.

An evaluation of this Freestore implementation is documented in Portuguese [here][thesis]. Eduardo Alchieri's original Freestore paper in Portuguese is here: [http://sbrc2014.ufsc.br/anais/files/trilha/ST07-2.pdf][freestore-article].

[![GoDoc](https://godoc.org/github.com/mateusbraga/freestore?status.png)](https://godoc.org/github.com/mateusbraga/freestore)
[![Build Status](https://travis-ci.org/mateusbraga/freestore.png?branch=master)](https://travis-ci.org/mateusbraga/freestore)

* [Freestore's Fault Tolerance Model](https://github.com/mateusbraga/freestore/blob/master/docs/fault-tolerance-model.md)
* [Mateus Braga's Bachelor's thesis (Portuguese)][thesis]
* [Eduardo Alchieri's original Freestore paper (Portuguese)][freestore-article]

[thesis]: http://www.mateusbraga.com.br/files/Monografia%20Mateus%20Antunes%20Braga.pdf
[freestore-article]: http://sbrc2014.ufsc.br/anais/files/trilha/ST07-2.pdf

## Development

Freestore is written in [Go](http://golang.org). You'll need a recent version of Go installed on your computer to build Freestore.

### Build

go build ./...

### Test

go test ./...

## Running

    freestore_server -bind :5000
    freestore_server -bind :5001
    freestore_server -bind :5002

    freestore_client

