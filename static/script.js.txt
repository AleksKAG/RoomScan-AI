async function generateDesign() {
    const style = document.getElementById('styleSelect').value;
    const id = document.getElementById('jobId').textContent;

    const res = await fetch(`/generate-design/${id}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: `style=${encodeURIComponent(style)}`
    });
    const data = await res.json();

    const gallery = document.getElementById('designGallery');
    gallery.innerHTML = '';
    data.designs.forEach(path => {
        const img = document.createElement('img');
        img.src = '/' + path.replace('/tmp/roomscan_uploads/', 'results/'); // нужно добавить FileServer для results
        img.style.width = '45%';
        img.style.margin = '10px';
        gallery.appendChild(img);
    });
}


function startAR() {
    navigator.mediaDevices.getUserMedia({video: true}).then(stream => {
        document.getElementById('arVideo').srcObject = stream;
        const ws = new WebSocket('ws://localhost:8080/ar-stream');
        ws.onmessage = (event) => {
            const url = URL.createObjectURL(event.data);
            document.getElementById('arVideo').src = url; // overlaid
        };
        // Send frames
        const canvas = document.createElement('canvas');
        setInterval(() => {
            canvas.width = 640;
            canvas.height = 480;
            canvas.getContext('2d').drawImage(document.getElementById('arVideo'), 0, 0, 640, 480);
            canvas.toBlob(blob => {
                blob.arrayBuffer().then(buf => ws.send(new Uint8Array(buf)));
            }, 'image/jpeg');
        }, 100); // every 100ms
    });
}
