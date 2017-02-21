FROM ubuntu:16.04
MAINTAINER Johannes M. Scheuermann <joh.scheuer@gmail.com>
ADD scheduler /scheduler
ENTRYPOINT ["/scheduler"]