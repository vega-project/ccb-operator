import axios from 'axios';

class Client {
    /**
     * Perform asynchronous request, using axios client.
     *
     * @param {String} method
     * @param {String} url
     * @param {Object} params
     * @param {Object} data
     *
     * @return {Promise}
     */
    request(method, url, params = {}, data) {
        const options = {
            method,
            url,
            params,
            data
        };

        return axios(options)
        .then((response) => response)
        .catch((error) => Promise.reject(`HTTP Client ${error}`));
    }
}

export default Client;
