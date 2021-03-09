const offerOptions = {
    offerToReceiveAudio: 1,
    offerToReceiveVideo: 1
};

document.addEventListener("DOMContentLoaded", function(event) {
    const inputID = document.querySelector(".input");
    const id = String(+new Date());

    start().then(localPeerConnection => {
        const socket = new WebSocket(`wss://${document.location.host}/ws?id=${id}`);
        socket.onopen = function(e) {
            console.log("[websocket] Connection established");
            socket.send(JSON.stringify({
                "from": id,
                "command": "connect"
            }));
        };

        socket.onmessage = function(event) {
            const payload = JSON.parse(event.data);
            console.log(`[websocket] Data received from server: `, payload.command);
            switch (payload.command) {
                case "connect":
                    createOffer(socket, localPeerConnection, payload);
                    break;
                case "offer":
                    createAnswer(socket, localPeerConnection, payload);
                    break;
                case "answer":
                    localPeerConnection.setRemoteDescription(new RTCSessionDescription(JSON.parse(payload.data)))
                    break;
                case "icecandidate":
                    const candidate = new RTCIceCandidate(JSON.parse(payload.data));
                    localPeerConnection.addIceCandidate(candidate);
                    break;
                case "disconnect":
                    const video = document.querySelector(`[data-user="${payload.from}"]`)
                    if (video) {
                        video.remove();
                    }
                    break;
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

        console.log("Peer connection created!")
    });
});

function onIceCandidate(socket, from, to) {
    return (event) => {
        try {
            if (event.candidate) {
                console.log(`ICE candidate`, event.candidate);
                socket.send(JSON.stringify({
                    "from": from,
                    "to": to,
                    "command": "icecandidate",
                    "data": JSON.stringify(event.candidate)
                }));
            }
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

    const localPeerConnection = new RTCPeerConnection({iceServers: [{urls: 'stun:stun.l.google.com:19302'}]});
    localPeerConnection.addEventListener('iceconnectionstatechange', e => onIceStateChange(localPeerConnection, e));

    localStream
        .getTracks()
        .forEach(track => localPeerConnection.addTrack(track, localStream));

    return localPeerConnection;
}

function onTrack(localPeerConnection, id) {
    localPeerConnection.ontrack = (event) => {
        console.log(`New track received: ${event.streams.length}`);
        if (event.streams.length > 0 && event.streams[0]) {
            const stream = event.streams[0];
            if (document.getElementById(stream.id)) {
                return
            }

            const video = document.createElement("video");
            video.setAttribute("data-user", id)
            video.id = stream.id;
            video.autoplay = true
            video.playsInline = true;
            video.controls = true;
            video.classList.add("column");
            video.srcObject = stream;
            document.querySelector(".videos").append(video);
        };
    };
}

function createOffer(socket, localPeerConnection, payload) {
    console.log("Creating offer to ", payload.from)
    localPeerConnection.onicecandidate = onIceCandidate(socket, payload.to, payload.from);
    localPeerConnection.createOffer(offerOptions).then(offer => {
        return localPeerConnection.setLocalDescription(offer);
    }).then(() => {
        return onTrack(localPeerConnection, payload.from);
    }).then(() => {
        socket.send(JSON.stringify({
            "from": payload.to,
            "to": payload.from,
            "command": "offer",
            "data": JSON.stringify(localPeerConnection.localDescription)
        }));
    });
}

function createAnswer(socket, localPeerConnection, payload) {
    console.log("Creating answer to ", payload.from)
    localPeerConnection.onicecandidate = onIceCandidate(socket, payload.to, payload.from);
    const description = new RTCSessionDescription(JSON.parse(payload.data));
    onTrack(localPeerConnection, payload.from);
    localPeerConnection.setRemoteDescription(description).then(() => {
        return localPeerConnection.createAnswer(offerOptions);
    }).then(answer => {
        return localPeerConnection.setLocalDescription(answer);
    }).then(() => {
        socket.send(JSON.stringify({
            "from": payload.to,
            "to": payload.from,
            "command": "answer",
            "data": JSON.stringify(localPeerConnection.localDescription)
        }));
    });
}

