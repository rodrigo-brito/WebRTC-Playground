let userStream;
const offerOptions = {
    offerToReceiveAudio: 1,
    offerToReceiveVideo: 1
};

document.addEventListener("DOMContentLoaded", function(event) {
    const connections = {};
    let id = new URLSearchParams(window.location.search).get('id');
    const localVideo = document.getElementById('localVideo');
    const labelVideo = document.getElementById('label');
    const btnConnect = document.getElementById("connect");
    const inputID = document.getElementById("id");
    const share = document.getElementById("share");

    if (!id) {
        id = String(+new Date());
    }

    inputID.setAttribute("value", id);
    labelVideo.innerText = id;
    inputID.addEventListener('keyup', (e) => {
        localVideo.setAttribute("data-user", inputID.value);
        labelVideo.innerText = inputID.value;
    })

    initUserMedia(localVideo).then(stream => {
        userStream = stream;
        btnConnect.removeAttribute("disabled");
        inputID.removeAttribute("disabled");
        btnConnect.addEventListener("click", () => {
            if (btnConnect.getAttribute("disabled")) {
                return;
            }

            connect(connections, inputID.value).then(() => {
                btnConnect.setAttribute("disabled", "disabled");
                inputID.setAttribute("disabled", "disabled");
                share.removeAttribute("disabled");
                btnConnect.innerText = "Online";
            }).catch(console.error);
        })

        share.addEventListener("click", toggleCameraScreen(localVideo, connections, (stream) => {
            userStream = stream; // update stream with new source
            if (localVideo.getAttribute("data-source") === "camera") {
                share.innerText = "Share Screen";
            } else {
                share.innerText = "Use Camera";
            }
        }));
    });
});

function connect(connections, id) {
    return new Promise(
        (resolve, reject) => {
            if (!id) {
                alert("please, define an username before connect");
                reject("no username");
                return
            }

            const socket = new WebSocket(`wss://${document.location.host}/ws?id=${id}`);
            socket.onopen = function(e) {
                console.log("[websocket] Connection established");
                socket.send(JSON.stringify({
                    "from": id,
                    "command": "connect",
                }));
                resolve();
            };

            socket.onmessage = function(event) {
                const payload = JSON.parse(event.data);
                console.log(`[websocket] Data received from server: `, payload.command);
                switch (payload.command) {
                    case "connect":
                        const peerConnectionOffer = createPeerConnection(connections, userStream, payload.from)
                        createOffer(socket, peerConnectionOffer, payload);
                        break;
                    case "offer":
                        const peerConnectionAnswer = createPeerConnection(connections, userStream, payload.from)
                        createAnswer(socket, peerConnectionAnswer, payload);
                        break;
                    case "answer":
                        if (connections[payload.from]){
                            connections[payload.from].setRemoteDescription(new RTCSessionDescription(JSON.parse(payload.data)))
                        }
                        break;
                    case "icecandidate":
                        if (connections[payload.from]){
                            const candidate = new RTCIceCandidate(JSON.parse(payload.data));
                            connections[payload.from].addIceCandidate(candidate);
                        }
                        break;
                    case "disconnect":
                        if (connections[payload.from]){
                            connections[payload.from] = undefined;
                        }
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
                connect.innerText = "Offline";
                connect.classList.remove("is-primary");
            };

            socket.onerror = function(error) {
                console.error(`[websocket/error] ${error.message}`);
                reject(error)
            };
        }
    );

}

function toggleCameraScreen(localVideo, connections, callback) {
    return () => {
        if (localVideo.getAttribute("data-source") === "camera") {

            // check browser support
            if (!navigator.mediaDevices || !'getDisplayMedia' in navigator.mediaDevices) {
                alert("browser not supported")
                return;
            }

            navigator.mediaDevices.getDisplayMedia({video: true})
                .then(stream => replaceVideoStream(connections, localVideo, stream, "screen"))
                .then(stream => callback(stream));

        } else {
            navigator.mediaDevices.getUserMedia({audio: true, video: true})
                .then(stream => replaceVideoStream(connections, localVideo, stream, "camera"))
                .then(stream => callback(stream));
        }
    }
}

function replaceVideoStream(connections, localVideo, stream, source) {
    localVideo.srcObject = stream;
    localVideo.setAttribute("data-source", source);
    Object.keys(connections).forEach(function(id) {
        if (!connections[id]) {
            return
        }

        connections[id].getSenders().forEach(async sender => {
            if (sender.track && sender.track.kind === 'video') {
                const track = stream.getTracks().find(track => track.kind === 'video');
                console.log("update stream track -> ", id, track)
                await sender.replaceTrack(track);
            }
        });
    });
    return stream
}

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

function onIceStateChange(peerConnection, event) {
    if (peerConnection) {
        console.log(`ICE state: ${peerConnection.iceConnectionState}`);
        console.log('ICE state change event: ', event);
    }
}

async function initUserMedia(localVideo) {
    const stream = await navigator.mediaDevices.getUserMedia({audio: true, video: true});
    localVideo.srcObject = stream;

    const videoTracks = stream.getVideoTracks();
    const audioTracks = stream.getAudioTracks();

    if (videoTracks.length > 0) {
        console.log(`Using video device: ${videoTracks[0].label}`);
    }
    if (audioTracks.length > 0) {
        console.log(`Using audio device: ${audioTracks[0].label}`);
    }

    return stream
}

function onTrack(localPeerConnection, id) {
    localPeerConnection.ontrack = (event) => {
        console.log(`New track received: ${event.streams.length}`);
        if (event.streams.length > 0 && event.streams[0]) {
            const stream = event.streams[0];
            if (document.getElementById(stream.id)) {
                return
            }

            const wrapper = document.querySelector(".video-wrapper").cloneNode(true);
            const video = wrapper.querySelector("video");
            
            video.id = stream.id;
            video.srcObject = stream;
            video.muted = false;
            wrapper.setAttribute("data-user", id)
            wrapper.querySelector(".label").innerText = id;
            document.querySelector(".videos").append(wrapper);
        };
    };
}

function createPeerConnection(connections, stream, to) {
    const localPeerConnection = new RTCPeerConnection({
        iceServers: [
            {urls: 'stun:stun.l.google.com:19302'}
        ]
    });

    localPeerConnection.addEventListener('iceconnectionstatechange', e => onIceStateChange(localPeerConnection, e));
    stream.getTracks().forEach(track => localPeerConnection.addTrack(track, stream));

    connections[to] = localPeerConnection

    return localPeerConnection;
}

function createOffer(socket, localPeerConnection, payload) {
    console.log("Creating offer to ", payload.from)
    localPeerConnection.onicecandidate = onIceCandidate(socket, payload.to, payload.from);
    onTrack(localPeerConnection, payload.from);
    localPeerConnection.createOffer(offerOptions).then(offer => {
        return localPeerConnection.setLocalDescription(offer);
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

