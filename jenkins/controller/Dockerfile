# Jenkins LTS 2.580.1 with JDK 21.
# Source: https://www.jenkins.io/changelog-stable/#v2.580.1
# Image tag jenkins/jenkins:2.580.1-jdk21 exists on Docker Hub.
# jenkinsci/docker release 2.580.1 builds that line with JDK 21.0.12.1+1.
FROM jenkins/jenkins:2.580.1-jdk21

USER root
# The repo bind mount is owned by the host user. Git refuses it unless the
# directory is marked safe. /etc/gitconfig survives the jenkins_home volume.
RUN git config --system --add safe.directory /pipelineiq \
 && git config --system --add safe.directory '*'
USER jenkins

COPY --chown=jenkins:jenkins plugins.txt /usr/share/jenkins/ref/plugins.txt
RUN jenkins-plugin-cli --plugin-file /usr/share/jenkins/ref/plugins.txt
