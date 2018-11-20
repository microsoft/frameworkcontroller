# MIT License
#
# Copyright (c) Microsoft Corporation. All rights reserved.
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE

FROM golang:alpine as builder

ENV PROJECT_DIR=${GOPATH}/src/github.com/microsoft/frameworkcontroller
ENV DIST_DIR=/frameworkcontroller

RUN apk update && apk add bash && mkdir -p ${PROJECT_DIR}
COPY . ${PROJECT_DIR}
RUN ${PROJECT_DIR}/build/frameworkbarrier/go-build.sh && \
  mv ${PROJECT_DIR} ${DIST_DIR}


FROM alpine:latest

ENV DIST_DIR=/frameworkcontroller

RUN apk update && apk add bash
COPY --from=builder ${DIST_DIR} ${DIST_DIR}
WORKDIR ${DIST_DIR}/dist/frameworkbarrier

ENTRYPOINT ["./start.sh"]
