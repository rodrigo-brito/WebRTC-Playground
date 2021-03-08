const offerOptions = {
    offerToReceiveAudio: 1,
    offerToReceiveVideo: 1
};

document.addEventListener("DOMContentLoaded", function(event) {
    let localPeerConnection;
    const socket = new WebSocket("wss://localhost:9000/ws");
    const inputID = document.querySelector(".input");
    const id = String(+new Date());
    const connections = {};
    inputID.setAttribute("value", id);


    socket.onopen = function(e) {
        console.log("[websocket] Connection established");
        socket.send(JSON.stringify({
            "from": inputID.getAttribute("value"),
            "command": "connect"
        }));
    };

    socket.onmessage = function(event) {
        const payload = JSON.parse(event.data);
        console.log(`[websocket] Data received from server: `, payload.command);
        switch (payload.command) {
            case "connect":
                // create offer
                connections[payload.from] = createOffer(socket, localPeerConnection, payload);
            case "offer":
                createAnswer(socket, localPeerConnection, payload);
            case "answer":
                // connect
        }
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

    start().then(conn => {
        localPeerConnection = conn;
        console.log("Peer connection created!")
    });
});

function onIceCandidate(local, remote) {
    return (connection, event) => {
        try {
            remote.addIceCandidate(event.candidate).then(() => {
                console.log(`ICE candidate:\n${event.candidate ? event.candidate.candidate : '(null)'}`);
            });
        } catch (e) {
            console.error(`failed to add ICE Candidate: ${e.toString()}`);
        }
    }
}

function onIceStateChange(pc, event) {
    if (pc) {
        console.log(`ICE state: ${pc.iceConnectionState}`);
        console.log('ICE state change event: ', event);
    }
}

async function start() {
    const localVideo = document.getElementById('localVideo');

    const stream = await navigator.mediaDevices.getUserMedia({audio: true, video: true});
    localVideo.srcObject = stream;
    const localStream = stream;

    const videoTracks = localStream.getVideoTracks();
    const audioTracks = localStream.getAudioTracks();

    if (videoTracks.length > 0) {
        console.log(`Using video device: ${videoTracks[0].label}`);
    }
    if (audioTracks.length > 0) {
        console.log(`Using audio device: ${audioTracks[0].label}`);
    }

    const localPeerConnection = new RTCPeerConnection({});
    localPeerConnection.addEventListener('iceconnectionstatechange', e => onIceStateChange(localPeerConnection, e));

    localStream
        .getTracks()
        .forEach(track => localPeerConnection.addTrack(track, localStream));

    return localPeerConnection;
}

function createOffer(socket, localPeerConnection, payload) {
    const remotePeerConnection = new RTCPeerConnection({});

    localPeerConnection.createOffer(offerOptions).then(offer => {
        socket.send(JSON.stringify({
            "from": payload.to,
            "to": payload.from,
            "command": "offer",
            "data": offer.sdp
        }));
    });

    localPeerConnection.onicecandidate = onIceCandidate(localPeerConnection, remotePeerConnection)
    return localPeerConnection
}

function createAnswer(socket, localPeerConnection, payload) {
     // TODO
}

