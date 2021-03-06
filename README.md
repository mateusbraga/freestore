# Freestore

[![GoDoc](https://godoc.org/github.com/mateusbraga/freestore?status.png)](https://godoc.org/github.com/mateusbraga/freestore)
[![Build Status](https://travis-ci.org/mateusbraga/freestore.png?branch=master)](https://travis-ci.org/mateusbraga/freestore)

Freestore is a research project developed as part of Mateus Braga's [Bachelor's thesis (Portuguese)][thesis] at University of Brasilia, Brazil. It is a library that implements a fault-tolerant, consistent, and reconfigurable distributed memory. 

Freestore implements a storage abstraction with READ/WRITE operations. It replicates the data over all servers and uses the idea that two majorities always intersect to provide fault-tolerance and per-key strong consistency (linearizability - read most recently written value). Freestore also implements a fault-tolerant reconfiguration mechanism (other than consensus protocol - no agreement is required, like CRDTs) that allows the cluster to continue operating correctly during configuration changes.

Freestore is by design a quorum system without a consensus protocol and as such cannot support conditional write operations like compare-and-set. This limits the kind of applications that should use something like Freestore to the ones that don't need to atomically perform conditional writes.

An evaluation of this Freestore implementation is documented in Portuguese [here][thesis]. Eduardo Alchieri's original Freestore paper in Portuguese is here: [http://sbrc2014.ufsc.br/anais/files/trilha/ST07-2.pdf][freestore-article].

* [Freestore's Fault Tolerance Model](https://github.com/mateusbraga/freestore/blob/master/docs/fault-tolerance-model.md)
* [Mateus Braga's Bachelor's thesis (Portuguese)][thesis]
* [Eduardo Alchieri's original Freestore paper (Portuguese)][freestore-article]
* [Static Quorum System implementation for comparison with Freestore](https://github.com/mateusbraga/static-quorum-system)
* [Dynastore implementation for comparison with Freestore](https://github.com/mateusbraga/dynastore)

[thesis]: http://www.mateusbraga.com.br/files/Monografia%20Mateus%20Antunes%20Braga.pdf
[freestore-article]: http://sbrc2014.ufsc.br/anais/files/trilha/ST07-2.pdf

## Development

Freestore is written in [Go](http://golang.org). You'll need a recent version of Go (at least go1.2) installed on your computer to build Freestore.

### Build/Install

go install ./...

### Test

go test ./...

## Running

    $GOPATH/bin/freestore_server -bind :5000
    $GOPATH/bin/freestore_server -bind :5001
    $GOPATH/bin/freestore_server -bind :5002

    $GOPATH/bin/freestore_client

