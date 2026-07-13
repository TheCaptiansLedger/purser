// k6 gRPC suite for ItemPersonService — a join-shaped entity keyed by the
// composite (itemId, personId, role). See test/k6/grpc/entry_person_test.js
// for the same pattern applied to EntryPerson.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const ADDR = __ENV.PURSER_GRPC_ADDR || 'localhost:7474';

const client = new grpc.Client();
client.load(['../../../proto'], 'purser/domain/v1/item_person.proto');

export default () => {
  client.connect(ADDR, { plaintext: true });

  const itemId = `k6-grpc-item-${__VU}-${__ITER}-${Date.now()}`;
  const personId = 'k6-person-1';
  const role = 'performer';

  let res = client.invoke('purser.domain.v1.ItemPersonService/CreateItemPerson', {
    itemPerson: { itemId: itemId, personId: personId, role: role, creditedAs: 'K6 Performer' },
  });
  check(res, {
    'CreateItemPerson status is OK': (r) => r && r.status === grpc.StatusOK,
    'CreateItemPerson returns the role': (r) => r && r.message && r.message.itemPerson && r.message.itemPerson.role === role,
  });

  res = client.invoke('purser.domain.v1.ItemPersonService/GetItemPerson', { itemId: itemId, personId: personId, role: role });
  check(res, {
    'GetItemPerson status is OK': (r) => r && r.status === grpc.StatusOK,
    'GetItemPerson returns the created credit': (r) => r && r.message && r.message.itemPerson && r.message.itemPerson.creditedAs === 'K6 Performer',
  });

  res = client.invoke('purser.domain.v1.ItemPersonService/UpdateItemPerson', {
    itemPerson: { itemId: itemId, personId: personId, role: role, creditedAs: 'K6 Performer Updated' },
    updateMask: 'creditedAs',
  });
  check(res, {
    'UpdateItemPerson status is OK': (r) => r && r.status === grpc.StatusOK,
    'UpdateItemPerson applied the field-masked credit': (r) =>
      r && r.message && r.message.itemPerson && r.message.itemPerson.creditedAs === 'K6 Performer Updated',
  });

  res = client.invoke('purser.domain.v1.ItemPersonService/ListItemPeople', { itemId: itemId, pageSize: 10 });
  check(res, {
    'ListItemPeople status is OK': (r) => r && r.status === grpc.StatusOK,
    'ListItemPeople includes the created credit': (r) =>
      r && r.message && r.message.itemPeople && r.message.itemPeople.some((ip) => ip.personId === personId && ip.role === role),
  });

  res = client.invoke('purser.domain.v1.ItemPersonService/DeleteItemPerson', { itemId: itemId, personId: personId, role: role });
  check(res, { 'DeleteItemPerson status is OK': (r) => r && r.status === grpc.StatusOK });

  res = client.invoke('purser.domain.v1.ItemPersonService/GetItemPerson', { itemId: itemId, personId: personId, role: role });
  check(res, { 'GetItemPerson after Delete is NotFound': (r) => r && r.status === grpc.StatusNotFound });

  client.close();
};
