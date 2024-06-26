# Copyright 2023 Dynatrace LLC

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#      https://www.apache.org/licenses/LICENSE-2.0

#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

import platform
import os

OS = platform.system().lower()

IS_WINDOWS = OS == "windows"

IS_DARWIN = OS == "darwin"

EXEC_EXTENSION = ""
if IS_WINDOWS:
    EXEC_EXTENSION = ".exe"

ARCHITECTURE = "amd64"

if platform.processor().lower() == "arm":
    ARCHITECTURE = "arm64"


def is_executable(path):
    return os.path.exists(path) and os.access(path, os.X_OK) and not os.path.isdir(path)
