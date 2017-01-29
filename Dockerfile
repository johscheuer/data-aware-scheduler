FROM scratch
MAINTAINER Johannes M. Scheuermann <joh.scheuer@gmail.com>
ADD scheduler /scheduler
ENTRYPOINT ["/scheduler"]