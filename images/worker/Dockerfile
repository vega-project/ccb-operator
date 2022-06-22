FROM centos:8

RUN yum install -y gcc-gnat-4.8.5 libgnat-4.8.5 libgnat-devel-4.8.5

ADD worker /usr/bin/worker
ADD bin/atlas12_ada /usr/bin/atlas12_ada
ADD bin/synspec49 /usr/bin/synspec49

ENTRYPOINT ["/usr/bin/worker"]