"""Verify Collection create/update request replays remain independent of reorder."""
import json
import uuid


def verify(request, sql, token, login_id):
    owner = sql("SELECT id FROM app.users WHERE login_id='" + login_id + "'")
    created_body = {'name': '並べ替え再送確認の架空設定', 'research_instruction': '初回保存の架空指示',
                    'initial_lookback_mode': 'ai', 'initial_lookback_days': None}
    create_key = str(uuid.uuid4())
    created_status, _, created_response = request('/api/collections', token, created_body,
        extra_headers={'Idempotency-Key': create_key})
    assert created_status == 201
    collection_id = json.loads(created_response)['id']
    detail = json.loads(request('/api/collections/' + collection_id, token)[2])
    update_body = {'name': '並べ替え後も維持する架空設定', 'research_instruction': '編集後の架空指示',
                   'initial_lookback_mode': 'days', 'initial_lookback_days': 21, 'version': detail['version']}
    update_key = str(uuid.uuid4())
    updated_status, _, updated_response = request('/api/collections/' + collection_id, token, update_body,
        extra_headers={'Idempotency-Key': update_key}, method='PATCH')
    assert updated_status == 200 and json.loads(updated_response)['version'] == '2'

    def settings_state():
        return sql("SELECT coalesce(json_agg(c ORDER BY id)::text,'[]') FROM app.collections c WHERE user_id='" + owner + "'")

    def history_state():
        tables = ('generation_jobs', 'generation_attempts', 'reports', 'report_target_results',
                  'report_facts', 'report_sources', 'fact_sources', 'report_source_failures')
        return {table: sql("SELECT coalesce(json_agg(t ORDER BY id)::text,'[]') FROM app." + table + ' t') for table in tables}

    before_order = json.loads(request('/api/home', token)[2])['collections']
    expected_order = [{'id': row['id'], 'version': row['version']} for row in before_order]
    ordered_ids = [row['id'] for row in reversed(before_order)]
    assert len(ordered_ids) > 1
    prior_state = settings_state()
    prior_history = history_state()
    order_key = str(uuid.uuid4())
    assert request('/api/collections/order', token,
        {'expected': expected_order, 'ordered_ids': ordered_ids},
        extra_headers={'Idempotency-Key': order_key}, method='PUT')[0] == 204
    after_order_state = settings_state()
    assert after_order_state != prior_state
    after_order_detail = request('/api/collections/' + collection_id, token)[2]
    after_order_home = json.loads(request('/api/home', token)[2])
    assert [row['id'] for row in after_order_home['collections']] == ordered_ids
    assert next(row for row in after_order_home['collections'] if row['id'] == collection_id)['name'] == update_body['name']

    replayed_update, _, replayed_update_body = request('/api/collections/' + collection_id, token, update_body,
        extra_headers={'Idempotency-Key': update_key}, method='PATCH')
    assert replayed_update == 200 and json.loads(replayed_update_body)['version'] == '2'
    assert request('/api/collections', token, created_body, extra_headers={'Idempotency-Key': create_key})[0] == 200
    assert settings_state() == after_order_state
    assert request('/api/collections/' + collection_id, token)[2] == after_order_detail
    replayed_home = json.loads(request('/api/home', token)[2])
    assert [row['id'] for row in replayed_home['collections']] == ordered_ids
    assert history_state() == prior_history
    assert sql("SELECT count(*) FROM app.collection_order_requests WHERE user_id='" + owner + "' AND request_key='" + order_key + "'") == '1'
    print('Independent Collection order fixture: old create/update request replay preserved current settings, version, order, and saved history: passed', flush=True)
