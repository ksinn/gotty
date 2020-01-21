FROM ubuntu:18.04
RUN useradd -ms /bin/bash student
ADD ./gotty /webtty/gotty
ADD ./resources/index.html /webtty/index.html
USER student
WORKDIR /home/student
