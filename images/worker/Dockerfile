FROM quay.io/centos/centos:centos7

RUN sed -i 's/mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-*
RUN sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*

RUN yum install -y gcc-gnat-4.8.5 libgnat-4.8.5 libgnat-devel-4.8.5 && \
    yum clean all

ADD worker /usr/bin/worker
ADD bin/atlas12_ada /usr/bin/atlas12_ada
ADD bin/synspec49 /usr/bin/synspec49

ENTRYPOINT ["/usr/bin/worker"]