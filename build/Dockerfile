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

FROM gcr.io/google-containers/debian-base-amd64:0.3

COPY build.sh /build.sh

ENV VERSION 1.4.4 
ENV SHA256 1f4c8fbd8a2a4309af7e3f9e8449625e3078ecb3e37eff5bd7b398d42756f58d 

RUN clean-install bash

RUN /build.sh
