# Fault Tolerance Model

This document is a fault tolerance specification of the system. The
intent is to get all of the assumptions out on the table. 

Each identified possible error is classified as one of the following:
untolerated, detected and tolerated. Also, each error's probability of
occurrence is estimated. Ideally, all untolerated errors should have
a negligible probability, all detected error should specify its
detection procedure, and all tolerated error should specify its masking
method. 

The classification of each error takes into account the system design,
the estimated probability, the cost of turning them into detected or
tolerated, and the potential consequences of its occurrence. It is
recommended to contain detected errors in fail-fast modules (modules
that reports at its interface that something has gone wrong as soon as posible).

## The System

The system comprises a higher-level application with a freestore client
(the user) and the Freestore register subsystem, which itself comprises
multiple platforms running the servers connected to a network.


## Overall system fault tolerance model

* error-free operation: All work goes according to expectations. The
user's Reads and Writes are performed and the system confirms the
operations by returning a status to the user.

* tolerated error: The user who has initiated an action notices that the
system failed by checking the status returned by the action. This error
indicate that the assumption that at least a majority of servers is
running and reachable is flawed. In this case, the action may have been
performed, but the user should stop using the distributed register at
this point.

* untolerated error: The system fails without the user checking the status
returned of an action, so the user does not realize that its action may
not have completed and that it should stop using the system.

The tolerated error specification means that the entire system is
fail-fast.

## Platform and network fault tolerance model

* error-free operation: The hardware and operating system all follow their
specifications.

* tolerated error: Something fails in the hardware or operating system.
The system is fail-fast: the hardware or operating system detects the
failure and restarts from a clean state before initiating any further
actions. This error is then handled as a power failure.

* untolerated error: Something fails in the hardware or operating system.
The processor muddles along and corrupted data is used before detecting
the failure. This error is similar to a byzantine error, which the
design does not try to detect and neither tolerate.

* untolerated error: Communication errors that cause the client or servers
to malfunction silently, like an error that the TCP error detection
mechanism does not detect and that also represents a valid RPC request
or response.

## FreeStore Client

### End-to-end layer 

The end-to-end layer masks divergence in the responses from the servers.

    Read() (value, error)
    Write(value) error

* error-free operation: Read returns the value of the last Write.

* tolerated error: A majority of servers disagree on their register's
value. Read masks this error by writing the correct value (the one
returned by Quorum-Read with the highest timestamp) to all servers
before returning the correct value. This is required to guarantee read
and write coherence. Write masks this error by using the returned value
from Quorum-Read as the one with the highest associated timestamp to
determine which is the next timestamp.  

* detected error: All RPC errors with more than 'floor((N-1)/2)'
processes. The failure to acquire the response from a majority of
processes is a failure of the system assumption that a majority of
processes is always working. This error is detected in the Quorum layer
and returned to the caller of Read an Write.

### Quorum layer

The quorum layer implements the N-modular redundancy and masks old view error.

    Quorum-Read(view) (value, error)
    Quorum-Write(view, value) error

* error-free operation: Quorum-Read returns the value with the highest
associated timestamp of the distributed register. Quorum-Write writes
a new value in the register of at least a majority of servers.

* tolerated error: All RPC errors in up to 'floor((N-1)/2)' processes.
Read and Write mask these error by using N-modular redundancy (it
implements a voter that uses the response of a majority).

* tolerated error: The Client has an old View of the system. Quorum-Read
and Quorum-Write mask this error by updating the clients view of the
system and then retrying the operation.

* detected error: All RPC errors with more than 'floor((N-1)/2)'
processes. The failure to acquire the response from a majority of
processes causes Quorum-Read and Quorum-Write to return an error. This
error is detected by counting the number of errors returned by the
communication service (RPC).

* detected error: A majority of servers disagree on the register's value.
Quorum-Read detect this error by comparing the received values from
a majority of servers and returning the value with the highest
associated timestamp along with an error. This error does not affect
Quorum-Write. 

## Freestore Communication module: RPC library

    SendRPCRequest(destination, serviceMethod, arg, *reply) error

* error-free operation: Call serviceMethod at destination with args and
return the result in "reply". 

* detected error: Communication errors. SendRPCRequest returns errors
returned from the RPC/TCP implementation.

  Examples of communication errors: The destination crashed or is
unreachable, serviceMethod signaled an error during its execution, and
any detected but not masked error according to the TCP specification and
Go's RPC implementation.

## Freestore Server's register

    Read() (value, error)
    Write(value) error


* tolerated error: Communication errors. The server will detect and report any invalid request and then discard it.

* detected error: Request to an old view. The server will compare its current view with the request associated view and return an error in the reply along with the newer view. Both in Read and in Write.

## Freestore Server's Reconfiguration module

* tolerated error: Communication errors. The server will detect and report any invalid
request and then discard it.

* detected error: Request to an old view. The server will compare its current view with the request associated view and return an error in the reply along with the newer view. Both in Read and in Write.