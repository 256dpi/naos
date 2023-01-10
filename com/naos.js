export const Types = {b: 'Bool', s: 'String', l: 'Long', d: 'Double', a: 'Action'};
export const Modes = {v: 'Volatile', s: 'System', a: 'Application', p: 'Public', l: 'Locked'};

export default class NAOS {
    timeout = 5000;
    debug = false;
    url = '';
    onConnect = () => {};
    onUpdate = () => {};
    onError = () => {};
    onDisconnect = () => {};

    constructor(url) {
        this.url = url;
        this.requests = {};
        this.connect();
    }

    async ping() {
        try {
            await this.send('ping');
        } catch (err) {
            console.log('ping', err);
            this.reconnect();
        }
    }

    async locked() {
        return await this.send('lock') === 'locked';
    }

    async unlock(password) {
        return await this.send('unlock', password) === 'unlocked';
    }

    async list() {
        let list = await this.send('list');
        return list.split(',').map(param => {
            return param.split(':');
        });
    }

    async read(name) {
        return await this.send('read:' + name);
    }

    async write(name, value) {
        await this.send('write:' + name, value);
    }

    connect() {
        this.socket = new WebSocket(this.url);
        this.socket.onopen = (event) => {
            if (this.debug) { console.log('onopen', event); }
            this.onConnect();
            this.interval = setInterval(() => {
                this.ping().then();
            }, this.timeout * 2);
        }
        this.socket.onmessage = (event) => {
            // if (this.debug) { console.log('onmessage', event); }
            let [type, value] = event.data.toString().split('#', 2);
            if (type.startsWith('update:')) {
                this.onUpdate(type.split(':')[1], value);
            } else {
                for (let resolve of this.requests[type] || []) {
                    resolve(value);
                }
                this.requests[type] = [];
            }
        }
        this.socket.onerror = (err) => {
            if (this.debug) { console.log('onerror', err); }
            this.onError(err);
        }
        this.socket.onclose = (event) => {
            if (this.debug) { console.log('onclose', event); }
            this.reconnect();
        }
    }

    send(type, value = undefined) {
        return new Promise((resolve, reject) => {
            this.socket.send(type + (value !== undefined ? '#' + value: ''));
            this.requests[type] ||= [];
            this.requests[type].push(resolve);
            setTimeout(() => {
                reject(new Error('timed out'));
            }, this.timeout);
        })
    }

    reconnect() {
        // stop timer
        clearInterval(this.interval);

        // terminate socket
        if (this.socket) {
            this.socket.onopen = null;
            this.socket.onmessage = null;
            this.socket.onerror = null;
            this.socket.onclose = null;
            this.socket.close();
            this.socket = null;
            this.onDisconnect();
        }

        // connect again
        setTimeout(() => {
            this.connect();
        });
    }
}
