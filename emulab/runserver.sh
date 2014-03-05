#!/bin/sh

HOSTNAME=`hostname`
#HOSTNAME=node-5.local
POINT_POSITION=`expr index "$HOSTNAME" '.'`
NODE_NUMBER_LENGTH=`expr $POINT_POSITION - 6`
NODE_NUMBER=${HOSTNAME:5:$NODE_NUMBER_LENGTH}

#echo $HOSTNAME
#echo $POINT_POSITION
#echo $NODE_NUMBER_LENGTH
#echo $NODE_NUMBER

BIND_ADDR=10.1.1.`expr $NODE_NUMBER + 1`:5000

#echo $BIND_ADDR

/home/mateus/go/bin/freestored -bind $BIND_ADDR $@
