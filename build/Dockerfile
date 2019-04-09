# Copyright 2015 The Kubernetes Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM quay.io/kubernetes-ingress-controller/debian-base-amd64:0.1

COPY build.sh /build.sh

ENV VERSION 2.0.15
ENV SHA256 8647dbef5475e6fc2738f4ac46b363de6ab7571f33743a493ad5ab5231b7d26a 

RUN clean-install bash

RUN /build.sh
