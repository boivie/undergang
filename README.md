# undergang

## Introduction

Undergang is a reverse HTTP proxy with websocket support, that connects to
web servers over SSH connections. Confusing?

It allows a browser to connect to HTTP servers running on backend servers,
which can only be connected to over SSH.

```
 Browser ==[ HTTP ]==> undergang ==[ SSH ]==> SSHD ==[ HTTP ]==> HTTP Server
```

## Usage

There is a configuration file you can edit.

## License

Copyright 2016 Victor Boivie <<victor@boivie.com>>

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
