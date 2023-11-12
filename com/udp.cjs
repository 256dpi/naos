const udp = require('dgram');

const client = udp.createSocket('udp4');

client.on('message', function(msg, info) {
  console.log('Data received from server : ' + msg.toString());
  console.log('Received %d bytes from %s:%d:\n', msg.length, info.address, info.port, msg);
});

const msg = new Uint8Array([1, 0, 0, 0, 11, 12, 12]);

client.send(msg, 8080, 'naos-247074609542548.local.', function(error) {
  if (error) {
    client.close();
  } else {
    console.log('Data sent !!!');
  }
});
