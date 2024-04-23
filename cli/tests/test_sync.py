from io import StringIO
import unittest
import datetime
import json
import responses
from responses import matchers
from unittest.mock import patch, mock_open
import sys
import os
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '../src')))  # Add parent directory to path

mock_plan_response = open('./tests/mock-plan-response.json', 'r')
mock_apply_response = open('./tests/mock-apply-response.json', 'r')

class TestScript(unittest.TestCase):
    @patch('builtins.open', new_callable=mock_open) # mute logs
    @patch('logging.basicConfig') # mute logs
    @patch('datetime.datetime')
    @patch('logging.info')
    @patch('logging.debug')
    @patch.dict('os.environ', {
        'DYNATRACE_SOURCE_API_TOKEN': 'source_token',
        'DYNATRACE_SOURCE_ENV_URL': 'source_url',
        'DYNATRACE_API_TOKEN': 'target_token',
        'DYNATRACE_ENV_URL': 'target_url'
    })
    @responses.activate
    def test_whole_script(
        self,
        mock_debug,
        mock_info,
        mock_datetime,
        mock_log_config, # mute logs
        mock_open # mute logs
        ):
        
        mock_datetime.now.return_value = datetime.datetime(2024, 3, 13, 10, 30, 15)
        mock_strftime = mock_datetime.now.return_value.strftime
        mock_strftime.return_value = '2024-03-13_10-30-15'  # mocked formatted string
        current_datetime = datetime.datetime.now()
        test_action_id = current_datetime.strftime('%Y-%m-%d_%H-%M-%S')
        
        tenant_list_body= {'tenantKey': {'Main': '0', 'Target': '1'}, 'tenants': {'0': {'label': 'SourceEnv', 'APIKey': 'source_token', 'url': 'source_url', 'notes': '', 'monacoConcurrentRequests': '10', 'disableSystemProxies': False, 'proxyURL': ''}, '1': {'label': 'Target', 'APIKey': 'target_token', 'url': 'target_url', 'notes': '', 'monacoConcurrentRequests': '10', 'disableSystemProxies': False, 'proxyURL': ''}}}
        tenant_list_url='http://localhost:3004/tenant_list'
        plan_url = "http://localhost:3004/migrate_settings_2_0?tenant_key_main=0&tenant_key_target=1&action_id=%s&enable_dashboards=true&enable_omit_destroy=false&enable_ultra_parallel=true&terraform_parallelism=10" % test_action_id
        apply_url = "http://localhost:3004/terraform_apply_all?tenant_key_main=0&tenant_key_target=1&action_id=%s"% test_action_id
        
        responses.post(tenant_list_url, match=[matchers.json_params_matcher(tenant_list_body)])
        responses.get(tenant_list_url, json=tenant_list_body)
        responses.post('http://localhost:3004/extract_configs?tenant_key=0', json='{}')
        responses.post('http://localhost:3004/extract_entity_v2?tenant_key=0&time_from_minutes=21600&time_to_minutes=0', json='{}')
        responses.post('http://localhost:3004/extract_configs?tenant_key=1', json='{}')
        responses.post('http://localhost:3004/extract_entity_v2?tenant_key=1&time_from_minutes=21600&time_to_minutes=0', json='{}')
        responses.post(plan_url,json=json.load(mock_plan_response))
        responses.post(apply_url, json=json.load(mock_apply_response))
        
        import sync # execute sync.py
        
        self.assertEqual(len(responses.calls), 8) # test received requests and order
        self.assertEqual(responses.calls[0].request.url, tenant_list_url)
        self.assertEqual(responses.calls[6].request.url, plan_url)
        self.assertEqual(responses.calls[7].request.url, apply_url)
        
        expected_log_info_calls = [
            'Sync procedure [2024-03-13_10-30-15] has started.',
            'Extracting entities and config for src cluster...',
            'Extracted entities and config for src cluster.',
            'Extracting entities and config for dst cluster...',
            'Extracted entities and config for dst cluster.',
            'Planning...',
            'Add 1 resources:  test_module >>> test_resource_4',
            'Update 1 resources:  test_module >>> test_resource_2',
            'Delete 1 resources:  test_module >>> test_resource_3',
            'Planned.',
            'Applying...',
            'ALL PLANNED CONFIG APPLIED',
            'Sync procedure is terminated.'
        ]
        expected_log_debug_calls = [
            'Loaded configuration',
            json.dumps(tenant_list_body)
        ]
        
        mock_info.assert_has_calls([unittest.mock.call(msg) for msg in expected_log_info_calls])
        mock_debug.assert_has_calls([unittest.mock.call(msg) for msg in expected_log_debug_calls])

if __name__ == '__main__':
    unittest.main()
