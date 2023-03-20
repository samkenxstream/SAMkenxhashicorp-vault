/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { parsePkiCert } from 'vault/utils/parse-pki-cert';
import ApplicationAdapter from './application';

export default ApplicationAdapter.extend({
  namespace: 'v1',

  url(snapshot, action) {
    const { backend, caType, type } = snapshot.attributes();
    if (action === 'sign-intermediate') {
      return `/v1/${backend}/root/sign-intermediate`;
    }
    if (action === 'set-signed-intermediate') {
      return `/v1/${backend}/intermediate/set-signed`;
    }
    if (action === 'upload') {
      return `/v1/${backend}/config/ca`;
    }
    return `/v1/${backend}/${caType}/generate/${type}`;
  },

  createRecordOrUpdate(store, type, snapshot, requestType) {
    const serializer = store.serializerFor('application');
    const isUpload = snapshot.attr('uploadPemBundle');
    const isSetSignedIntermediate = snapshot.adapterOptions.method === 'setSignedIntermediate';
    let action = snapshot.adapterOptions.method === 'signIntermediate' ? 'sign-intermediate' : null;
    let data;
    if (isUpload) {
      action = 'upload';
      data = { pem_bundle: snapshot.attr('pemBundle') };
    } else if (isSetSignedIntermediate) {
      action = 'set-signed-intermediate';
      data = { certificate: snapshot.attr('certificate') };
    } else {
      data = serializer.serialize(snapshot, requestType);

      // The type parameter is serialized but is part of the URL. This means
      // we'll get an unknown parameter warning back from the server if we
      // send it. Remove it instead.
      delete data.type;
    }

    return this.ajax(this.url(snapshot, action), 'POST', { data }).then((response) => {
      // uploading CA, setting signed intermediate cert, and attempting to generate
      // a new CA if one exists, all return a 204
      if (!response) {
        response = {};
      }
      response.id = snapshot.id;
      response.modelName = type.modelName;
      // only parse if certificate is attached to response
      if (response.data && response.data.certificate) {
        const caCertMetadata = parsePkiCert(response.data);
        const transformedResponse = { ...response, ...caCertMetadata };
        store.pushPayload(type.modelName, transformedResponse);
      } else {
        store.pushPayload(type.modelName, response);
      }
    });
  },

  createRecord() {
    return this.createRecordOrUpdate(...arguments);
  },

  updateRecord() {
    return this.createRecordOrUpdate(...arguments);
  },
});
