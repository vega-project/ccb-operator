FROM fedora:31

RUN yum install -y gcc-gnat libgnat libgnat-devel && \
    yum clean all

ADD worker /usr/bin/worker
ADD bin/atlas12_ada /usr/bin/atlas12_ada
ADD bin/synspec49 /usr/bin/synspec49

ENTRYPOINT ["/usr/bin/worker"]