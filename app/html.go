package app

var script = `
    var conn;
    if (window["WebSocket"]) {
        var wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        conn = new WebSocket(wsProtocol + "//" + window.location.hostname + ":" + window.location.port + window.location.pathname.substring(0, window.location.pathname.lastIndexOf('/')) + "/__undergang_02648018bfd74fa5a4ed50db9bb07859_ws");
        conn.onclose = function(evt) {
		console.log("Connection closed");
        }
        conn.onmessage = function(evt) {
            console.log(evt)
            var payload = JSON.parse(evt.data)
            if (payload.kind == "connection_success") {
                window.location.reload(false);
            }
        }
    } else {
        console.log("Your browser doesn't support javascript.");
    }
`

var contents = `
<!DOCTYPE html>
<html lang="en">
<head>
<title>Preparing...</title>
<script src="//ajax.googleapis.com/ajax/libs/jquery/2.0.3/jquery.min.js"></script>
<script src="__undergang_02648018bfd74fa5a4ed50db9bb07859_script.js"></script>
<style type="text/css">
/* Loader #1 by Sam Lillicrap
   http://www.samueljwebdesign.co.uk
   http://codepen.io/samueljweb/pen/LbGxi
*/
@import url(//fonts.googleapis.com/css?family=Lato:100,300,700);
html {
  background-color: {{.BackgroundColor}};
}

h1 {
  font-family: 'Lato';
  color: white;
  text-transform: uppercase;
  font-size: 1em;
  letter-spacing: 1.5px;
  text-align: center;
  width: 155px;
  margin-top: 20px;
  -webkit-animation: fade 2s infinite;
  -moz-animation: fade 2s infinite;
}

#container {
  width: 180px;
  padding-top: 180px;
  margin: auto;
  vertical-align: middle;
}

.stick {
  width: 30px;
  height: 3px;
  background: white;
  display: inline-block;
  margin-left: -8px;
}

.stick:nth-child(n) {
  transform: rotate(30deg);
  -ms-transform: rotate(30deg);
  /* IE 9 */
  -webkit-transform: rotate(30deg);
  /* Safari and Chrome */
  -moz-transform: rotate(30deg);
  -webkit-animation: fall 2s infinite;
  -moz-animation: fall 2s infinite;
}

.stick:nth-child(2n) {
  transform: rotate(-30deg);
  -ms-transform: rotate(-30deg);
  /* IE 9 */
  -webkit-transform: rotate(-30deg);
  /* Safari and Chrome */
  -moz-transform: rotate(-30deg);
  -webkit-animation: rise 2s infinite;
  -moz-animation: rise 2s infinite;
}

@-webkit-keyframes rise {
  50% {
    transform: rotate(30deg);
    -ms-transform: rotate(30deg);
    /* IE 9 */
    -webkit-transform: rotate(30deg);
    -moz-transform: rotate(30deg);
  }
}
@-moz-keyframes rise {
  50% {
    transform: rotate(30deg);
    -ms-transform: rotate(30deg);
    /* IE 9 */
    -webkit-transform: rotate(30deg);
    -moz-transform: rotate(30deg);
  }
}
@-o-keyframes rise {
  50% {
    transform: rotate(30deg);
    -ms-transform: rotate(30deg);
    /* IE 9 */
    -webkit-transform: rotate(30deg);
    -moz-transform: rotate(30deg);
  }
  @keyframes rise {
    50% {
      transform: rotate(30deg);
      -ms-transform: rotate(30deg);
      /* IE 9 */
      -webkit-transform: rotate(30deg);
      -moz-transform: rotate(30deg);
    }
  }
}
@-webkit-keyframes fall {
  50% {
    transform: rotate(-30deg);
    -ms-transform: rotate(-30deg);
    /* IE 9 */
    -webkit-transform: rotate(-30deg);
    -moz-transform: rotate(30deg);
  }
}
@-moz-keyframes fall {
  50% {
    transform: rotate(-30deg);
    -ms-transform: rotate(-30deg);
    /* IE 9 */
    -webkit-transform: rotate(-30deg);
    -moz-transform: rotate(-30deg);
  }
}
@-o-keyframes fall {
  50% {
    transform: rotate(-30deg);
    -ms-transform: rotate(-30deg);
    /* IE 9 */
    -webkit-transform: rotate(-30deg);
    -moz-transform: rotate(30deg);
  }
  @keyframes fall {
    50% {
      transform: rotate(-30deg);
      -ms-transform: rotate(-30deg);
      /* IE 9 */
      -webkit-transform: rotate(-30deg);
      -moz-transform: rotate(30deg);
    }
  }
}
@-webkit-keyframes fade {
  50% {
    opacity: 0.5;
  }
  100% {
    opacity: 1;
  }
}
@-moz-keyframes fade {
  50% {
    opacity: 0.5;
  }
  100% {
    opacity: 1;
  }
}
@-o-keyframes fade {
  50% {
    opacity: 0.5;
  }
  100% {
    opacity: 1;
  }
  @keyframes fade {
    50% {
      opacity: 0.5;
    }
    100% {
      opacity: 1;
    }
  }
}

#log {
	font-family: 'Lato';
	color: white;
    overflow: auto;
}
</style>
</head>
<body>

<div id="container">
  <div class="stick"></div>
  <div class="stick"></div>
  <div class="stick"></div>
  <div class="stick"></div>
  <div class="stick"></div>
  <div class="stick"></div>

  <h1>Preparing, please wait...</h1>

</div>

<div id="log"></div>
</body>
</html>`
