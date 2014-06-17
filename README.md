# Freestore

Freestore is a library that implements a consistent and reconfigurable distributed memory. 

Freestore implements a Storage abstraction with READ/WRITE operations. It replicates the data on all servers and uses the idea that two majorities always intersect to provide fault-tolerance and sequential consistency. Freestore also implements a reconfiguration mechanism that allows the cluster to continue operating normally during configuration changes.

This Freestore implementation is the result of Mateus Braga's [Bachelor's thesis (PT-BR)][thesis] at University of Brasilia, Brazil. Eduardo Alchieri's Freestore original paper in Portuguese is here: [http://sbrc2014.ufsc.br/anais/files/trilha/ST07-2.pdf][freestore-article].

[![GoDoc](https://godoc.org/github.com/mateusbraga/freestore?status.png)](https://godoc.org/github.com/mateusbraga/freestore)
[![Build Status](https://travis-ci.org/mateusbraga/freestore.png?branch=master)](https://travis-ci.org/mateusbraga/freestore)

[thesis]: http://www.mateusbraga.com.br/files/Monografia%20Mateus%20Antunes%20Braga.pdf
[freestore-article]: http://sbrc2014.ufsc.br/anais/files/trilha/ST07-2.pdf

# Fault Tolerance Model

[Freestore's Fault Tolerance Model](https://github.com/mateusbraga/freestore/blob/master/docs/fault-tolerance-model.md)
