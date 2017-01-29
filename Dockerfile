FROM scratch
MAINTAINER Johannes M. Scheuermann <joh.scheuer@gmail.com>
ADD data /scheduler
ENTRYPOINT ["/scheduler"]
