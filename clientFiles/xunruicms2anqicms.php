<?php
define('IS_INDEX', true);
define('URL_BIND', 'anqicms');

define('XUNRUI_PATH', dirname(__FILE__) . '/../');
define('XUNRUI_CONFIG_PATH', XUNRUI_PATH . 'config/');
define('XUNRUI_FCPATH', XUNRUI_PATH . 'dayrui/');
define('XUNRUI_ROOTPATH', XUNRUI_PATH);
define('XUNRUI_WRITEPATH', XUNRUI_PATH . 'cache/');
if (!empty($_GET['debug'])) {
    ini_set("display_errors", 1);
    error_reporting(E_ALL & ~E_NOTICE);
} else {
    ini_set("display_errors", 0);
    error_reporting(0);
}
if (version_compare(PHP_VERSION, '5.3.0', '<')) {
    showMessage('PHP版本过低', 1003);
}
if (!class_exists('PDO', false)) {
    showMessage("不支持pdo", 1003);
}
define('VERSION', '1.0.0');
define('ANQI_PATH', dirname(__FILE__) . '/');
$client = new xunrui2anqi();
$client->run();
class xunrui2anqi
{
    private $configPath;
    private $config = array();
    private $action;
    private $db;
    private $dbPrefix;
    private $currentTable;
    private $currentWhere;
    private $currentOrder;
    private $currentLimit;
    private $currentField;
    public function __construct()
    {
        @set_error_handler(array(&$this, 'error_handler'), E_ALL & ~E_NOTICE & ~E_WARNING);
        @set_exception_handler(array(&$this, 'exception_handler'));
        $this->action = isset($_GET['a']) ? $_GET['a'] : '';
        $from = isset($_GET['from']) ? $_GET['from'] : '';
        if ($from != 'anqicms') {
            showMessage("接口文件访问正常");
        }
        $this->configPath = ANQI_PATH . 'anqicms.config.php';
        if (file_exists($this->configPath)) {
            $this->config = include($this->configPath);
        }
        $this->verify();
    }
    function verify() {
        $checkToken = isset($_GET['token']) ? $_GET['token'] : '';
        $checkTime  = isset($_GET['_t']) ? $_GET['_t'] : '';
        if (md5($this->config['token'] . $checkTime) != $checkToken && $this->action != 'config') {
            res(1001, "访问受限");
        }
    }
    public function error_handler($type, $message, $file, $line)
    {
        $msg = $message.' '.$file.' '.$line;
        self::whetherOut($msg, $type);
    }
    public function exception_handler($error)
    {
        $msg = $error->getMessage().' '.$error->getFile().' '.$error->getLine().' ';
        self::whetherOut($msg, $error->getCode());
    }
    private static function whetherOut($str,$type=30719){
        if (intval($type) <= 30719) {
            res(-1, $str);
        }
    }
    public function run()
    {
        if ($this->action == 'config') {
            $this->{$this->action}();
        }
        if (empty($this->config['checked'])) {
            res(1002, "接口配置异常");
        }
        $funcName = $this->action . "Action";
        if (!method_exists($this, $funcName)) {
            res(-1, '错误的入口');
        }
        $this->initDB();
        $this->{$funcName}();
    }
    function initDB() {
        $dbConfig = $this->getXunruiDBConfig();
        try {
            $dsn = "mysql:host={$dbConfig['hostname']};dbname={$dbConfig['database']};charset=utf8mb4";
            $this->db = new PDO($dsn, $dbConfig['username'], $dbConfig['password']);
            $this->db->setAttribute(PDO::ATTR_ERRMODE, PDO::ERRMODE_EXCEPTION);
            $this->db->setAttribute(PDO::ATTR_DEFAULT_FETCH_MODE, PDO::FETCH_OBJ);
            $this->dbPrefix = $dbConfig['DBPrefix'];
        } catch (PDOException $e) {
            res(-1, '数据库连接失败: ' . $e->getMessage());
        }
    }
    
    function getXunruiDBConfig() {
        $configFile = XUNRUI_CONFIG_PATH . 'database.php';
        if (!file_exists($configFile)) {
            res(-1, '迅睿CMS数据库配置文件不存在');
        }
        $db = [];
        require $configFile;
        if (empty($db['default'])) {
            res(-1, '迅睿CMS数据库配置格式错误');
        }
        return $db['default'];
    }
    
    function getXunruiSiteId() {
        try {
            $sql = "SELECT id FROM {$this->dbPrefix}site LIMIT 1";
            $stmt = $this->db->prepare($sql);
            $stmt->execute();
            $result = $stmt->fetch(PDO::FETCH_OBJ);
            return $result ? intval($result->id) : 1;
        } catch (PDOException $e) {
            return 1;
        }
    }
    
    function table($tableName) {
        $this->currentTable = $tableName;
        $this->currentWhere = '';
        $this->currentOrder = '';
        $this->currentLimit = '';
        $this->currentField = '*';
        return $this;
    }
    
    function field($fields) {
        $this->currentField = is_array($fields) ? implode(',', $fields) : $fields;
        return $this;
    }
    
    function where($condition) {
        $this->currentWhere = $condition;
        return $this;
    }
    
    function order($order) {
        $this->currentOrder = $order;
        return $this;
    }
    
    function limit($limit) {
        $this->currentLimit = "LIMIT " . intval($limit);
        return $this;
    }
    
    function decode() {
        return $this;
    }
    
    function select() {
        $sql = "SELECT {$this->currentField} FROM {$this->currentTable}";
        if ($this->currentWhere) {
            $sql .= " WHERE {$this->currentWhere}";
        }
        if ($this->currentOrder) {
            $sql .= " ORDER BY {$this->currentOrder}";
        }
        if ($this->currentLimit) {
            $sql .= " {$this->currentLimit}";
        }
        
        try {
            $stmt = $this->db->prepare($sql);
            $stmt->execute();
            return $stmt->fetchAll();
        } catch (PDOException $e) {
            return [];
        }
    }
    
    function find() {
        $this->currentLimit = "LIMIT 1";
        $results = $this->select();
        return !empty($results) ? $results[0] : null;
    }
    public function syncDataAction() {
        $type = isset($_GET['type']) ? $_GET['type'] : '';
        $siteId = intval(isset($_GET['target_id']) ? $_GET['target_id'] : $this->getXunruiSiteId());
        $prefix = $this->dbPrefix . $siteId . "_";
        switch($type) {
            case 'module':
                $modules = array();
                try {
                    $moduleTable = $this->dbPrefix . 'module';
                    $sql = "SELECT id, dirname, site FROM {$moduleTable} WHERE disabled = 0 ORDER BY id ASC";
                    $stmt = $this->db->prepare($sql);
                    $stmt->execute();
                    $moduleRows = $stmt->fetchAll(PDO::FETCH_OBJ);
                    
                    foreach ($moduleRows as $mod) {
                        $moduleName = $mod->dirname;
                        $mainTable = $prefix . $moduleName;
                        $moduleId = $mod->id;
                        if ($moduleId == 6) {
                            $moduleId = 1;
                        }
                        if ($this->tableExists($mainTable)) {
                            $fields = $this->getModuleFields($mod->id);
                            $categoryFields = $this->getCategoryFields($mod->id);
                            $modules[] = array(
                                'id' => intval($moduleId),
                                'table_name' => $moduleName,
                                'name' => ucfirst($moduleName),
                                'title' => ucfirst($moduleName),
                                'is_system' => 0,
                                'title_name' => '',
                                'status' => 1,
                                'fields' => $fields,
                                'category_fields' => $categoryFields,
                            );
                        }
                    }
                } catch (PDOException $e) {
                }
                res(0, '', $modules);
                break;
            case 'category':
                $module = isset($_GET['module']) ? $_GET['module'] : 'share';
                if (!$module) {
                    res(0, '', array());
                }
                
                $catTable = $prefix . "share_category";
                if (!$this->tableExists($catTable)) {
                    res(0, '', array());
                }
                
                $fields = array('*');
                $where = "disabled = 0 AND `show` = 1 AND tid = 1";
                if ($module != 'share') {
                    $where .= " AND mid = '" . $module . "'";
                }
                
                $cats = $this->table($catTable)->field($fields)->where($where)->order('id asc')->select();
                
                $categoryFields = $this->getCategoryFields(0);
                $list = array();
                foreach ($cats as $key => $val) {
                    $cid = intval(isset($val->id) ? $val->id : 0);
                    $pid = intval(isset($val->pid) ? $val->pid : 0);
                    $title = isset($val->name) ? $val->name : '';
                    $dirname = isset($val->dirname) ? $val->dirname : '';
                    $moduleId = $this->getModuleId($val->mid);
                    if ($moduleId == 6) {
                        $moduleId = 1;
                    }
                    $content = html_entity_decode($val->content);
                    $description = str_replace("\n", " ", strip_tags($content));
                    if(strlen($description) > 250) {
                        $description = mb_substr($description, 0, 250);
                    }
                    $extra = array();
                    foreach ($categoryFields as $field) {
                        $extra[$field['field_name']] = $this->parseFieldValue(isset($val->{$field['field_name']}) ? $val->{$field['field_name']} : '', $field['type'], $field['old_type'], $field["setting"]);
                    }
                    $images = [];
                    $thumbs = json_decode($val->thumb, true);
                    if (isset($thumbs['file'])) {
                        foreach ($thumbs['file'] as $tval) {
                            $tmp = $this->fixUrl($tval);
                            if ($tmp) {
                                $images[] = $tmp;
                            }
                        }
                    }
                    $setting = json_decode($val->setting, true);
                    $seo_title = $setting['seo']['list_title'];
                    $keywords = $setting['seo']['list_keywords'];
                    if ($setting['seo']['list_description']) {
                        $description = $setting['seo']['list_description'];
                    }
                    
                    $item = array(
                        'id' => $cid,
                        'parent_id' => $pid,
                        'title' => $title,
                        'description' => $description,
                        'content' => $content,
                        'status' => 1,
                        'type' => 1,
                        'logo' => count($images) > 0 ? $images[0] : '',
                        'images' => $images,
                        'sort' => intval(isset($val->displayorder) ? $val->displayorder : 0),
                        'url_token' => $dirname,
                        'seo_title' => $seo_title,
                        'keywords' => $keywords,
                        'module_id' => $moduleId,
                        'extra' => (object)$extra,
                    );
                    $list[] = $item;
                }
                res(0, '', $list);
                break;
            case 'tag':
                res(0, '', array());
                break;
            case 'keyword':
                res(0, '', array());
                break;
            case 'archive':
                $lastId = intval(isset($_GET['last_id']) ? $_GET['last_id'] : 0);
                $lastMod = isset($_GET['last_mod']) ? $_GET['last_mod'] : '';
                $limit = 200;
                $moduleId = 0;
                $moduleIndex = 0;
                
                try {
                    $moduleTable = $this->dbPrefix . 'module';
                    $sql = "SELECT id, dirname, site FROM {$moduleTable} WHERE disabled = 0 ORDER BY id ASC";
                    $stmt = $this->db->prepare($sql);
                    $stmt->execute();
                    $modules = $stmt->fetchAll(PDO::FETCH_OBJ);
                    if ($lastMod == "") {
                        $lastMod = $modules[0]->dirname;
                    }
                    
                    $moduleName = "";
                    foreach ($modules as $key => $mod) {
                        if ($mod->dirname == $lastMod) {
                            $moduleName = $mod->dirname;
                            $moduleId = intval($mod->id);
                            $moduleIndex = $key;
                            break;
                        }
                    }
                    
                    if (!$moduleName) {
                        res(0, '', array());
                        break;
                    }
                    
                    $mainTable = $prefix . $moduleName;
                    if ($this->tableExists($mainTable)) {
                        $archives = $this->table($mainTable)->where("id>" . $lastId . " AND status = 9")->order("id asc")->limit($limit)->select();
                    }
                    
                    if (empty($archives)) {
                        for ($i = 0; $i < count($modules); $i++) {
                            $moduleIndex++;
                            if (isset($modules[$moduleIndex])) {
                                $lastMod = $modules[$moduleIndex]->dirname;
                                $moduleId = intval($modules[$moduleIndex]->id);
                                $moduleName = $modules[$moduleIndex]->dirname;
                                $mainTable = $prefix . $moduleName;
                                $lastId = 0;
                                $archives = $this->table($mainTable)->where("id>" . $lastId . " AND status = 9")->order("id asc")->limit($limit)->select();
                                if (!empty($archives)) {
                                    break;
                                }
                            } else {
                                break;
                            }
                        }
                    }
                    $fields = $this->getModuleFields($moduleId);
                    $archiveList = array();
                    foreach ($archives as $key => $val) {
                        $id = intval($val->id);
                        $catid = intval(isset($val->catid) ? $val->catid : 0);
                        $title = isset($val->title) ? $val->title : '';
                        $keywords = isset($val->keywords) ? $val->keywords : '';
                        $description = isset($val->description) ? $val->description : '';
                        $ctime = isset($val->inputtime) ? $val->inputtime : (isset($val->updatetime) ? $val->updatetime : '');
                        $utime = isset($val->updatetime) ? $val->updatetime : $ctime;
                        $content = '';
                        $images = array();
                        $thumbs = json_decode($val->thumb, true);
                        if (isset($thumbs['file'])) {
                            foreach ($thumbs['file'] as $tval) {
                                $tmp = $this->fixUrl($tval);
                                if ($tmp) {
                                    $images[] = $tmp;
                                }
                            }
                        } else if (is_string($val->thumb)) {
                            $tmp = $this->fixUrl($val->thumb);
                            if ($tmp) {
                                $images[] = $tmp;
                            }
                        }
                        $tableid = isset($val->tableid) ? intval($val->tableid) : 0;
                        $extra = array();
                        if ($tableid >= 0) {
                            $dataTable = $prefix . $moduleName . "_data_" . $tableid;
                            if ($this->tableExists($dataTable)) {
                                $d = $this->table($dataTable)->where("id=" . $id)->find();
                                if ($d) {
                                    if (isset($d->content)) {
                                        $content = $d->content;
                                    } elseif (isset($d->content_1)) {
                                        $content = $d->content_1;
                                    }
                                }
                            }
                        }
                        $content = html_entity_decode($content);
                        $description = html_entity_decode($description);
                        foreach ($fields as $field) {
                            if (isset($val->{$field['field_name']})) {
                                $tmpField = $val->{$field['field_name']};
                            } else if (isset($d) && isset($d->{$field['field_name']})) {
                                $tmpField = $d->{$field['field_name']};
                            }
                            $extra[$field['field_name']] = $this->parseFieldValue($tmpField, $field['type'], $field['old_type'], $field["setting"]);
                        }
                        
                        if ($moduleId == 6) {
                            $moduleId = 1;
                        }
                        $archiveList[$key] = array(
                            'id' => $id,
                            'title' => $title,
                            'keywords' => $keywords,
                            'description' => $description,
                            'category_id' => $catid,
                            'views' => intval(isset($val->hits) ? $val->hits : 0),
                            'status' => 1,
                            'created_time' => is_numeric($ctime) ? intval($ctime) : (strtotime($ctime) ?: time()),
                            'updated_time' => is_numeric($utime) ? intval($utime) : (strtotime($utime) ?: time()),
                            'images' => $images,
                            'url_token' => '',
                            'module_id' => intval($moduleId),
                            'flag' => '',
                            'content' => $content,
                            'extra' => (object)$extra,
                        );
                    }
                    
                    res(0, '', $archiveList, array("last_mod" => $lastMod));
                } catch (PDOException $e) {
                    res(0, '', array());
                }
                break;
            case 'singlepage':
                $catTable = $prefix . "share_category";
                if (!$this->tableExists($catTable)) {
                    res(0, '', array());
                }
                
                $fields = array('*');
                $where = "disabled = 0 AND `show` = 1 AND tid = 0";
                
                try {
                    $cats = $this->table($catTable)->field($fields)->where($where)->order('pid asc, displayorder asc, id asc')->select();
                    
                    $list = array();
                    foreach ($cats as $key => $val) {
                        $cid = intval(isset($val->id) ? $val->id : 0);
                        $pid = intval(isset($val->pid) ? $val->pid : 0);
                        $title = isset($val->name) ? $val->name : '';
                        $dirname = isset($val->dirname) ? $val->dirname : '';
                        $content = html_entity_decode($val->content);
                        $description = strip_tags($content);
                        if(strlen($description) > 250) {
                            $description = mb_substr($description, 0, 250);
                        }
                        $images = [];
                        $thumbs = json_decode($val->thumb, true);
                        if ($thumbs['file']) {
                            foreach ($thumbs['file'] as $tval) {
                                $tmp = $this->fixUrl($tval);
                                if ($tmp) {
                                    $images[] = $tmp;
                                }
                            }
                        }
                        $item = array(
                            'id' => $cid,
                            'parent_id' => $pid,
                            'title' => $title,
                            'description' => $description,
                            'content' => $content,
                            'status' => 1,
                            'type' => 3,
                            'logo' => count($images) > 0 ? $images[0] : '',
                            'images' => $images,
                            'sort' => intval(isset($val->displayorder) ? $val->displayorder : 0),
                            'url_token' => $dirname,
                            'seo_title' => isset($val->seo_title) ? $val->seo_title : '',
                            'keywords' => isset($val->keywords) ? $val->keywords : '',
                            'module_id' => 0,
                        );
                        $list[] = $item;
                    }
                    res(0, '', $list);
                } catch (PDOException $e) {
                    res(0, '', array());
                }
                break;
            case 'static':
                $file = ANQI_PATH.'anqitmp.zip';
                if(!file_exists($file)) {
                    $dir = ANQI_PATH.'uploadfile';
                    $this->create_zip(rtrim(ANQI_PATH, "/"), $dir, $file);
                }
                $lastId = isset($_GET['last_id']) ? intval($_GET['last_id']) : 0;
                $limit = 1048576 * 3;
                $fileSize = filesize($file);
                if($fileSize <= $lastId) {
                    die("@end");
                }
                $handle = fopen($file, 'r');
                if($lastId > 0) {
                    fseek($handle, $lastId);
                }
                $source = fread($handle, $limit);
                fclose($handle);
                echo $source;
                die;
                break;
        }
    }

    function getModuleId($module) {
        static $moduleIds = array();
        if (isset($moduleIds[$module])) {
            return $moduleIds[$module];
        }
        $moduleTable = $this->dbPrefix . 'module';
        $sql = "SELECT id, dirname, site FROM {$moduleTable} WHERE dirname = :dirname ORDER BY displayorder ASC, id ASC limit 1";
        $stmt = $this->db->prepare($sql);
        $stmt->execute(array(':dirname' => $module));
        $row = $stmt->fetch(PDO::FETCH_OBJ);
        $moduleIds[$module] = $row ? intval($row->id) : 0;
        return $moduleIds[$module];
    }
    
    function getModuleFields($moduleId) {
        $fields = array();
        try {
            $fieldTable = $this->dbPrefix . 'field';
            $sql = "SELECT * FROM {$fieldTable} WHERE relatedname = 'module' AND relatedid = :moduleId AND issystem = 0 AND disabled = 0 ORDER BY displayorder ASC, id ASC";
            $stmt = $this->db->prepare($sql);
            $stmt->execute(array(':moduleId' => $moduleId));
            $rows = $stmt->fetchAll(PDO::FETCH_OBJ);
            
            foreach ($rows as $row) {
                $setting = json_decode(str_replace('\r', '', $row->setting), true);
                $option = isset($setting['option']) ? $setting['option'] : array();
                $fields[] = array(
                    'name' => isset($row->name) ? $row->name : '',
                    'field_name' => isset($row->fieldname) ? $row->fieldname : '',
                    'required' => false,
                    'is_filter' => false,
                    'content' => isset($option['options']) ? $option['options'] : (isset($option['value']) ? $option['value'] : ''),
                    'type' => $this->mapFieldType($row->fieldtype),
                    'old_type' => $row->fieldtype,
                    "setting" => $row->setting,
                );
            }
        } catch (PDOException $e) {
        }
        return $fields;
    }
    
    function getCategoryFields($moduleId) {
        $fields = array();
        try {
            $fieldTable = $this->dbPrefix . 'field';
            $sql = "SELECT * FROM {$fieldTable} WHERE relatedname = 'category-share' AND issystem = 0 AND disabled = 0 ORDER BY displayorder ASC, id ASC";
            $stmt = $this->db->prepare($sql);
            $stmt->execute();
            $rows = $stmt->fetchAll(PDO::FETCH_OBJ);
            
            foreach ($rows as $row) {
                $setting = json_decode(str_replace('\r', '', $row->setting), true);
                $option = isset($setting['option']) ? $setting['option'] : array();
                $fields[] = array(
                    'name' => isset($row->name) ? $row->name : '',
                    'field_name' => isset($row->fieldname) ? $row->fieldname : '',
                    'required' => false,
                    'is_filter' => false,
                    'content' => isset($option['options']) ? $option['options'] : (isset($option['value']) ? $option['value'] : ''),
                    'type' => $this->mapFieldType($row->fieldtype),
                    'old_type' => $row->fieldtype,
                    "setting" => $row->setting,
                );
            }
        } catch (PDOException $e) {
        }
        return $fields;
    }
    
    function mapFieldType($fieldtype) {
        $typeMap = array(
            'Text' => 'text',
            'Textbtn' => 'text',
            'Textselect' => 'select',
            'Textarea' => 'textarea',
            'Editor' => 'editor',
            'Radio' => 'radio',
            'Select' => 'select',
            'Selects' => 'checkbox',
            'Checkbox' => 'checkbox',
            'Color' => 'color',
            'Date' => 'date',
            'Time' => 'time',
            'Diy' => 'textarea',
            'File' => 'file',
            'Files' => 'images',
            'Group' => 'textarea',
            'Linkage' => 'category',
            'Linkages' => 'category',
            'Touchspin' => 'number',
            'Property' => 'texts',
            'Uid' => 'number',
            'Pay' => 'textarea',
            'Pays' => 'textarea',
            'Paystext' => 'textarea',
            'Image' => 'image',
            'Images' => 'images',
            'Ftable' => 'texts',
            'Cat' => 'category',
            'Cats' => 'category',
            'Related' => 'archive',
            'Score' => 'textarea',
            'Members' => 'textarea',
            'Merge' => 'texts',
            'Redirect' => 'text',
            'Catids' => 'category',
        );
        return isset($typeMap[$fieldtype]) ? $typeMap[$fieldtype] : 'text';
    }

    function parseFieldValue($value, $type, $old_type, $typeSetting = '') {
        $option = json_decode($typeSetting ? $typeSetting : '', true);
        $option = $option['option'] ? $option['option'] : [];
        $value = str_replace("\r", '', $value);
        switch ($type) {
            case 'checkbox':
                $value = json_decode($value, true);
                $result = [];
                $tmpOpts = explode("\n", $option['options']);
                $options = [];
                foreach ($tmpOpts as $item) {
                    $tmp = explode('|', $item);
                    $options[$tmp[1]] = $tmp[0];
                }
                foreach ($value as $item) {
                    if ((is_string($item) || is_numeric($item)) && isset($options[$item])) {
                        $result[] = $options[$item];
                    } else if (!is_array($item)) {
                        $result[] = $item;
                    }
                }
                return implode(",", $result);
            case 'images':
                $value = json_decode($value, true);
                $result = [];
                if (isset($value['file'])) {
                    foreach ($value['file'] as $item) {
                        $result[] = $this->fixUrl($item);
                    }
                }
                return json_encode($result);
            case 'image':
            case 'file':
                $value = $this->fixUrl($value);
                break;
            case 'texts':
                $value = json_decode($value, true);
                $result = [];
                if ($old_type == "Property" && $value) {
                    foreach ($value as $item) {
                        $result[] = [
                            'key' => $item['name'],
                            'value' => $item['value'],
                        ];
                    }
                } else if ($old_type == "Ftable" && $value) {
                    $tmpOpts = $option['field'];
                    $options = [];
                    foreach ($tmpOpts as $key => $item) {
                        $options[$key] = $item['name'];
                    }
                    
                    foreach ($value as $item) {
                        foreach ($item as $key => $val) {
                            $result[] = [
                                'key' => $options[$key] ? $options[$key] : $key,
                                'value' => $val,
                            ];
                        }
                    }
                }
                return json_encode($result);
        }

        return $value;
    }
    
    function tableExists($table) {
        try {
            $sql = "SELECT 1 FROM {$table} LIMIT 1";
            $stmt = $this->db->prepare($sql);
            $stmt->execute();
            return true;
        } catch (PDOException $e) {
            return false;
        }
    }

    function fixUrl($path) {
        if (!$path) return $path;
        if (is_array($path)) {
            if (isset($path['file'])) {
                $path = $path['file'][0];
            } else {
                $path = array_values($path)[0];
            }
        } else if (is_string($path)) {
            $decoded = json_decode($path, true);
            if (isset($decoded['file'])) {
                $path = $decoded['file'][0];
            }
        }
        if (is_numeric($path)) {
            try {
                $attachTable = $this->dbPrefix . 'attachment_data';
                $sql = "SELECT attachment FROM {$attachTable} WHERE id = " . intval($path) . " LIMIT 1";
                $stmt = $this->db->prepare($sql);
                $stmt->execute();
                $result = $stmt->fetch(PDO::FETCH_OBJ);
                if ($result && $result->attachment) {
                    $path = $result->attachment;
                    // add prefix
                } else {
                    $path = '';
                }
            } catch (PDOException $e) {
                $path = '';
            }
        }
        //if (strpos($path, 'http') === 0 || $path == '') return $path;
        // $base = isset($this->config['base_url']) ? $this->config['base_url'] : baseUrl();
        // return rtrim($base, '/') . '/' . ltrim($path, '/');
        return $path;
    }
    function create_zip($baseDir, $sourceDir, $zipFile) {
        $zip = new ZipArchive();
        if ($zip->open($zipFile, ZipArchive::CREATE | ZipArchive::OVERWRITE) === TRUE)
        {
            $this->addFileToZip($zip, $baseDir, $sourceDir);
            $zip->close();
        } else {
        }
    }
    function addFileToZip(&$zip, $baseDir, $targetDir)
    {
        $dh     = opendir($targetDir);
        while ($file = readdir($dh))
        {
            if($file != "." and $file != "..")
            {
                $path = $targetDir."/".$file;
                if(is_dir($path))
                {
                    $this->addFileToZip($zip, $baseDir, $path);
                }
                elseif(is_file($path))
                {
                    $relPath = str_replace($baseDir . "/", '', $path);
                    $zip->addFile($path, $relPath);
                }
            }
        }
        closedir($dh);
    }
    private function config()
    {
        if (!is_array($this->config)) {
            $this->config = array();
        }
        if (isset($this->config['checked']) && $this->config['checked']) {
            res(0, "已配置过");
        }
        $config = json_decode(file_get_contents("php://input"), true);
        if (empty($config)) {
            $config = array();
        }
        foreach ($config as $key => $item) {
            if (is_array($item)) {
                if (!isset($this->config[$key]) || !is_array($this->config[$key])) {
                    $this->config[$key] = array();
                }
                foreach ($item as $k => $v) {
                    $this->config[$key][$k] = $v;
                }
            } else {
                $this->config[$key] = $item;
            }
        }
        $this->checkConfig();
        if (empty($this->config['checked'])) {
            res(1002, "配置失败");
        }
        res(0, "配置成功", $this->config);
    }
    private function checkConfig()
    {
        if (!isset($this->config['base_url']) || !$this->config['base_url']) {
            $this->config['base_url'] = baseUrl();
        } else {
            $this->config['base_url'] = rtrim($this->config['base_url'], "/") . "/";
        }
        $this->config['checked'] = $this->config['token'] ? true : false;
        $this->writeConfig();
    }
    private function writeConfig()
    {
        $configString = "<?php\n\nreturn " . var_export($this->config, true) . ";\n";
        $result = file_put_contents($this->configPath, $configString);
        if (!$result) {
            res(1002, "无法写入配置");
        }
    }
}
function res($code, $msg = null, $data = null, $extra = null)
{
    @header('Content-Type:application/json;charset=UTF-8');
    if(is_array($msg)){
        $msg = implode(",", $msg);
    }
    $output = array(
        'code' => $code,
        'msg'  => $msg,
        'data' => $data
    );
    if (is_array($extra)) {
        foreach ($extra as $key => $val) {
            $output[$key] = $val;
        }
    }
    echo json_encode($output);
    die;
}
function showMessage($msg, $code = -1)
{
    $from = isset($_GET['from']) ? $_GET['from'] : '';
    if ($from == 'x') {
        res($code, $msg);
    }
    @header('Content-Type:text/html;charset=UTF-8');
    echo "<div style='padding: 50px;text-align: center;'>$msg</div>";
    die;
}
function baseUrl()
{
    static $baseUrl;
    if ($baseUrl) {
        return $baseUrl;
    }
    $isHttps = false;
    if (!empty($_SERVER['HTTPS']) && strtolower($_SERVER['HTTPS']) !== 'off') {
        $isHttps = true;
    } elseif (isset($_SERVER['HTTP_X_FORWARDED_PROTO']) && $_SERVER['HTTP_X_FORWARDED_PROTO'] === 'https') {
        $isHttps = true;
    } elseif (!empty($_SERVER['HTTP_FRONT_END_HTTPS']) && strtolower($_SERVER['HTTP_FRONT_END_HTTPS']) !== 'off') {
        $isHttps = true;
    }
    $dirNameArr = explode('/', $_SERVER['REQUEST_URI']);
    array_pop($dirNameArr);
    $dirName = implode("/", $dirNameArr) . "/";
    $baseUrl = ($isHttps ? "https" : "http") . "://" . $_SERVER["HTTP_HOST"] . $dirName;
    return $baseUrl;
}
