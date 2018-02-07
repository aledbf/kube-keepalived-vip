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

ENV VERSION 31a172ff035ee957a54d49713aa42d3c9b58ad84
ENV SHA256 f113c3e435166ea7ff6d8d6e98ece9080acbeb6f988b3c86517d7c251a693cc2

RUN clean-install bash

RUN /build.sh
