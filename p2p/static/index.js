document.addEventListener("DOMContentLoaded", function(event) {
    const socket = new WebSocket("wss://localhost:9000/ws");
    const inputID = document.querySelector(".input");
    inputID.setAttribute("value", +new Date());


    socket.onopen = function(e) {
        console.log("[websocket] Connection established");
        socket.send(JSON.stringify({
            "from": inputID.getAttribute("value"),
            "command": "connect"
        }));
    };

    socket.onmessage = function(event) {
        console.log(`[websocket] Data received from server: `, event.data);
    };

    socket.onclose = function(event) {
        if (event.wasClean) {
            console.log(`[websocket] Connection closed cleanly, code=${event.code} reason=${event.reason}`);
        } else {
            // e.g. server process killed or network down
            // event.code is usually 1006 in this case
            console.log('[websocket] Connection died');
        }
    };

    socket.onerror = function(error) {
        console.error(`[websocket/error] ${error.message}`);
    };
});
