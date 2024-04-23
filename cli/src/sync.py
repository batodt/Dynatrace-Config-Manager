import requests
import datetime
import json
import os
import logging
import traceback

# Python 2 Compatible - Please don't use format string #

backend_baseurl = "http://localhost:3004"
statusDict = {'A':'Add', 'U': 'Update', 'D': 'Delete'}
action_id = datetime.datetime.now().strftime('%Y-%m-%d_%H-%M-%S')
main_log_filename = "./logs/%s-main.log" % (action_id)

logging.basicConfig(filename=main_log_filename,level=logging.DEBUG,format='%(asctime)s %(levelname)s %(message)s')

def log_planned_operation(status, kind):
    report = ""
    counter = 0
    for module in status['modules']:
        if kind in module['stats']:
            report = "%s %s >>>" % (report, module['module'])
            for resource in module['data']:
                if resource['status'] == kind:
                    counter += 1
                    report = "%s %s" % (report, resource['key_id'])
    logging.info("%s %d resources: %s" % (statusDict[kind], counter, report))

def execute_phase(name, payload, query):
    response = requests.post("%s/%s" % (backend_baseurl, name), data=json.dumps(payload), params=query, headers={'Content-Type':'application/json'})
    if response.status_code != 200:
        filename="%s-%s-ERROR.log" % (action_id, name)
        error_msg=" %s in phase %s - see response %s" % (response.status_code, name, filename)
        logging.error(error_msg)
        write_log_file(filename, str(response.content))
        raise Exception(error_msg)
    return response

def write_log_file(name, content):
    with open("logs/%s" % name, "w") as text_file: text_file.write(str(content))

try:
    print("Watch logs here: %s" % main_log_filename)
    logging.info("Sync procedure [%s] has started." % (action_id))
    tenant_list_body = {
        'tenantKey':{'Main':'0','Target':'1'},
        'tenants':{
            '0':{'label':'SourceEnv','APIKey':os.environ['DYNATRACE_SOURCE_API_TOKEN'],'url':os.environ['DYNATRACE_SOURCE_ENV_URL'],'notes':'','monacoConcurrentRequests':'10','disableSystemProxies':False,'proxyURL':''},
            '1':{'label':'Target','APIKey':os.environ['DYNATRACE_API_TOKEN'],'url':os.environ['DYNATRACE_ENV_URL'],'notes':'','monacoConcurrentRequests':'10','disableSystemProxies':False,'proxyURL':''}
        }
    }
    execute_phase("tenant_list", tenant_list_body, {})

    response = requests.get(backend_baseurl+"/tenant_list")
    logging.debug("Loaded configuration")
    logging.debug(response.text)

    logging.info("Extracting entities and config for src cluster...")
    src_extract_config_param = {'tenant_key' : '0'}
    result = execute_phase("extract_configs", {}, src_extract_config_param).json()
    filename="%s-config-src.log" % (action_id)
    write_log_file(filename, result)
    src_extract_entity_param = {'tenant_key':'0','time_from_minutes':'21600','time_to_minutes':'0'}
    result = execute_phase("extract_entity_v2", {}, src_extract_entity_param).json()
    filename="%s-entity-src.log" % (action_id)
    write_log_file(filename, result)
    logging.info("Extracted entities and config for src cluster.")

    logging.info("Extracting entities and config for dst cluster...")
    dst_extract_config_param = {'tenant_key' : '1'}
    result = execute_phase("extract_configs", {}, dst_extract_config_param).json()
    filename="%s-config-dst.log" % (action_id)
    write_log_file(filename, result)
    dst_extract_entity_param = {'tenant_key':'1','time_from_minutes':'21600','time_to_minutes':'0'}
    result = execute_phase("extract_entity_v2", {}, dst_extract_entity_param).json()
    filename="%s-entity-dst.log" % (action_id)
    write_log_file(filename, result)
    logging.info("Extracted entities and config for dst cluster.")

    logging.info("Planning...")
    migrate_setting_ot_tc_param = {'tenant_key_main':'0','tenant_key_target':'1','action_id':action_id,'enable_dashboards':'true','enable_omit_destroy':'false','enable_ultra_parallel':'true','terraform_parallelism':'10'}
    plan = execute_phase("migrate_settings_2_0", {}, migrate_setting_ot_tc_param).json()
    filename="%s-plan.log" % (action_id)
    write_log_file(filename, plan)
    log_planned_operation(plan, "A")
    log_planned_operation(plan, "U")
    log_planned_operation(plan, "D")
    logging.info("Planned.")

    logging.info("Applying...")
    push_all_configs_param = {'tenant_key_main':0,'tenant_key_target':1, 'action_id':action_id}
    push_all_configs_body = [{"module":"All","module_trimmed":"All","unique_name":"All"}]
    status = execute_phase("terraform_apply_all", push_all_configs_body, push_all_configs_param).json()
    if status["log_dict"]["apply_complete"] == True:
        log_file = "%s-apply-all.log" % (action_id)
        logging.info("ALL PLANNED CONFIG APPLIED")
        write_log_file(log_file, status)
        logging.info("Sync procedure is terminated.")
    else:
        log_file = "%s-apply-all.log" % (action_id)
        error_msg = "CONFIG APPLY FAILED OR PARTIAL - see" % log_file
        print(error_msg)
        logging.error(error_msg)
        write_log_file(log_file, status)
        raise Exception(error_msg)
except Exception as e:
    error_msg = "An error occurred: %s" % e
    logging.error(error_msg)
    logging.debug(traceback.print_exc())