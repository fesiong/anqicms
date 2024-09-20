<?php
/**
 * wordpress 数据转 anqicms 接口文件
 * 仅支持 php 5.3 以上
 * 版权保护，如需使用，请访问 https://www.anqicms.com/。
 * @author anqicms
 * 微信：websafety
 */

//报错设置
if (!empty($_GET['debug'])) {
    ini_set("display_errors", 1);
    error_reporting(E_ALL & ~E_NOTICE);
} else {
    ini_set("display_errors", 0);
    error_reporting(0);
}

//检测php版本
if (version_compare(PHP_VERSION, '5.3.0', '<')) {
    showMessage('当前PHP版本为'.phpversion().'，小于5.3,请升级php版本5.3以上', 1003);
}

//检测 空间是否支持 pdo_mysql
if (!class_exists('PDO', false)) {
    showMessage("空间不支持pdo连接", 1003);
}

define('VERSION', '1.0.1');
define('APP_PATH', __DIR__ . '/');

$client = new anqicms();
$client->run();

class anqicms
{
    private $configPath;
    private $config = array();
    private $action;
    private $db;

    public function __construct()
    {
        @set_error_handler(array(&$this, 'error_handler'), E_ALL & ~E_NOTICE & ~E_WARNING);
        @set_exception_handler(array(&$this, 'exception_handler'));

        $this->action = $_GET['a'];
        //来路
        $from = $_GET['from'];

        //用户正常访问提示
        if ($from != 'anqicms') {
            showMessage("接口文件访问正常");
        }

        $this->configPath = APP_PATH . 'anqicms.config.php';
        if (file_exists($this->configPath)) {
          $this->config = include($this->configPath);
        }

        // 先检查配置
        $this->verify();
    }

    function verify() {
      $checkToken = $_GET['token'];
      $checkTime  = $_GET['_t'];
      
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
            //配置异常
            res(1002, "接口配置异常，无法正常读取配置信息");
        }
        
        
        $funcName = $this->action . "Action";
        if (!method_exists($this, $funcName)) {
            res(-1, '错误的入口');
        }

        $this->initDB();

        $this->{$funcName}();
    }

    function initDB() {
        $this->db = new pdoMysql($this->config['database']);
        if (!$this->db) {
            res(-1, '链接数据库失败');
        }
    }

    /**
     * 需要执行的操作type：
     * -. 同步模型 module
     * -. 同步分类 category
     * -. 同步标签 tag
     * -. 同步锚文本 keyword
     * -. 同步文档 archive
     * -. 同步单页 singlepage
     * -. 同步静态资源 static
     */
    public function syncDataAction() {
      $type = $_GET['type'];
      
      switch($type) {
          case 'module':
            // wordpress 没有模型，默认是 1
            $modules = [];

            res(0, '', $modules);
            break;
        case 'category':
            $taxonomy = $this->db->select("*", "term_taxonomy", "taxonomy = 'category'");
            $terms = $this->db->select("*", "terms");
            $categories = array();
            foreach($taxonomy as $key => $val) {
                foreach($terms as $inner) {
                    if($val['term_id'] == $inner['term_id']) {
                        $val['name'] = $inner['name'];
                        $val['slug'] = $inner['slug'];
                        break;
                    }
                }

              $categories[] = array(
                'id' => $val['term_id'],
                'parent_id' => $val['parent'],
                'title' => $val['name'],
                'description' => $val['description'],
                'content' => '',
                'status' => 1,
                'type' => 1,
                'url_token' => $val['slug'],
                'module_id' => 1,
              );
            }
              
            res(0, '', $categories);
            break;
        case 'tag':
            $taxonomy = $this->db->select("*", "term_taxonomy", "taxonomy = 'post_tag'");
            $terms = $this->db->select("*", "terms");
            $tags = array();
            foreach($taxonomy as $key => $val) {
                foreach($terms as $inner) {
                    if($val['term_id'] == $inner['term_id']) {
                        $val['name'] = $inner['name'];
                        $val['slug'] = $inner['slug'];
                        break;
                    }
                }

              $tags[] = array(
                'id' => $val['term_id'],
                'title' => $val['name'],
                'description' => $val['description'],
                'url_token' => $val['slug'],
              );
            }
            
            res(0, '', $tags);
            break;
        
          case 'keyword':
            $keywords = [];
            
            res(0, '', $keywords);
            break;
        case 'archive':
            $lastId = intval($_GET['last_id']);
            $limit = 100;

            $archives = $this->db->select("*", "posts", "post_type = 'post' and post_status = 'publish' and ID > " . $lastId, $limit, "ID asc");
            foreach($archives as $key => $val) {
                $categoryId = 0;
                $tags = array();
                $relations = $this->db->select("*", "term_relationships", "object_id = " . $val['ID']);
                $term_taxonomy_ids = array();
                foreach ($relations as $inner) {
                    $term_taxonomy_ids[] = $inner['term_taxonomy_id'];
                }
                if (!empty($term_taxonomy_ids)) {
                    $taxonomy = $this->db->select("*", "term_taxonomy", "taxonomy = 'category' and term_taxonomy_id IN(".implode(',', $term_taxonomy_ids).")");
                    if(!empty($taxonomy)) {
                        $categoryId = $taxonomy[0]['term_id'];
                    }
                    $taxonomy = $this->db->select("*", "term_taxonomy", "taxonomy = 'post_tag' and term_taxonomy_id IN(".implode(',', $term_taxonomy_ids).")");
                    if(!empty($taxonomy)) {
                        $tagIds = array();
                        foreach($taxonomy as $inner) {
                            $tagIds[] = $inner['term_id'];
                        }
                        $terms = $this->db->select("*", "terms", "term_id IN(".implode(',', $tagIds).")");
                        foreach($terms as $inner) {
                            $tags[] = $inner['name'];
                        }
                    }
                }

              $url_token = $val['post_name'];
              if (strlen($url_token) > 150) {
                $url_token = substr($url_token, 0, 150);
              }
              $archive = array(
                'id' => $val['ID'],
                'title' => $val['post_title'],
                'url_token' => $url_token,
                'created_time' => strtotime($val['post_date']),
                'updated_time' => strtotime($val['post_modified']),
                'content' => $val['post_content'],
                'category_id' => $categoryId,
                'tags' => $tags,
                'module_id' => 1,
                'status' => 1,
              );
              $archive['content'] = $this->parseContent($archive['content']);
              $archive = $this->getThumb($archive);
              $archives[$key] = $archive;
            }
            
            res(0, '', $archives);
            break;
        case 'singlepage':
            $singlepages = $this->db->select("*", "posts", "post_type = 'page' and post_status = 'publish'");
            foreach($singlepages as $key => $val) {
              $val['post_content'] = $this->parseContent($val['post_content']);
              $singlepages[$key] = array(
                'id' => $val['ID'],
                'title' => $val['post_title'],
                'url_token' => $val['post_name'],
                'created_time' => strtotime($val['post_date']),
                'content' => $val['post_content'],
                'type' => 3,
                'status' => 1,
              );
            }
            
            res(0, '', $singlepages);
            break;
        case 'static':
            // 打包静态文件，包括模板静态文件、上传的文件
            $file = APP_PATH.'anqitmp.zip';
            if(!file_exists($file)) {
              $dir = APP_PATH.'wp-content/uploads';
                $this->create_zip(rtrim(APP_PATH, "/"), $dir, $file);
            }
            $lastId = $_GET['last_id'];
            // 1次1m
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

    /**
     * 内容处理
     *
     * @param [type] $content
     * @return void
     */
    private function parseContent($content) {
        if(empty($content)){
            return $content;
        }

        // 新版的WordPress 有不使用p标签的，这里增加检测
        $content = nl2p($content);

        $content = str_replace(array('[video','][/video]','mp4="'),array('<video','></video>','src="'),$content);

        $content = str_replace(array("\r", "\n", "\t"), '', $content);
        $content = str_replace('&middot;', '·', $content);
        $content = str_replace('&hellip;', '…', $content);
        $content = str_replace('&ldquo;', '“', $content);
        $content = str_replace('&rdquo;', '”', $content);
        
        return $content;
    }

    private function getThumb($article) {
        $baseDir = $this->setting['base_url'] . 'wp-content/uploads/';
        //查找 _thumbnail_id
        $thumbId = $this->db->getOneCol("meta_value", "postmeta", "meta_key = '_thumbnail_id' and post_id = '{$article['id']}'");
        //处理默认缩略图等
        if ($thumbId) {
            //查找 _wp_attachment_metadata
            $thumb = $this->db->getOneCol("meta_value", "postmeta", "meta_key = '_wp_attachment_metadata' and post_id = '{$thumbId}'");
            if ($thumb) {
                $thumb = unserialize($thumb);
                $article['images']  = [$baseDir . $thumb['file']];
            }
        }

        return $article;
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

    /**
     * 配置接口
     *
     * @return void
     */
    private function config()
    {
        if (!is_array($this->config)) {
            $this->config = array();
        }
        if ($this->config['checked']) {
          res(0, "已配置过，不能重复配置，如需重新配置，请手动删除anqicms.config.php");
        }

        $config = json_decode(file_get_contents("php://input"), true);
        if (empty($config)) {
            $config = array();
        }
        foreach ($config as $key => $item) {
            if (is_array($item)) {
                if (!is_array($this->config[$key])) {
                    $this->config[$key] = array();
                }
                foreach ($item as $k => $v) {
                    $this->config[$key][$k] = $v;
                }
            } else {
                $this->config[$key] = $item;
            }
        }

        //检查配置是否正确
        $this->checkConfig();

        if (empty($this->config['checked'])) {
            res(1002, "配置失败，无法正常连接数据库");
        }

        res(0, "配置成功", $this->config);
    }

    private function checkConfig()
    {
        global $checkToken, $checkTime;

        if (!$this->config['base_url']) {
            $this->config['base_url'] = baseUrl();
        } else {
            $this->config['base_url'] = rtrim($this->config['base_url'], "/") . "/";
        }

        $this->checkWordpress();

        $this->writeConfig();
    }

    private function checkWordpress()
    {
        if (empty($this->config['database'])) {
            $configFile = APP_PATH . "wp-config.php";
            if (!file_exists($configFile)) {
                res(1002, "接口配置异常，无法正常读取配置信息");
            }
            $table_prefix = '';
            $contents = file_get_contents($configFile);
            preg_match('/\$table_prefix\s*=\s*[\'|"](.+)[\'|"]/', $contents, $match);
            $table_prefix = $match[1];
            preg_match('/define\(\s*[\'|"]DB_NAME[\'|"],\s*[\'|"](.+)[\'|"]/', $contents, $match);
            $dbName = $match[1];
            preg_match('/define\(\s*[\'|"]DB_USER[\'|"],\s*[\'|"](.+)[\'|"]/', $contents, $match);
            $dbUser = $match[1];
            preg_match('/define\(\s*[\'|"]DB_PASSWORD[\'|"],\s*[\'|"](.+)[\'|"]/', $contents, $match);
            $dbPassword = $match[1];
            preg_match('/define\(\s*[\'|"]DB_HOST[\'|"],\s*[\'|"](.+)[\'|"]/', $contents, $match);
            $dbHost = $match[1];
            preg_match('/define\(\s*[\'|"]DB_CHARSET[\'|"],\s*[\'|"](.+)[\'|"]/', $contents, $match);
            $dbCharset = $match[1];

            $hostArr = explode(":", $dbHost);
            $this->config['database'] = array(
                'host'     => $hostArr[0],
                'port'     => empty($hostArr[1]) ? '3306' : $hostArr[1],
                'user'     => $dbUser,
                'password' => $dbPassword,
                'database' => $dbName,
                'charset'  => $dbCharset,
                'prefix'   => $table_prefix
            );
        }

        //写入一些配置
        $this->config['checked'] = $this->config['token'] ? true : false;
    }

    private function writeConfig()
    {
        $configString = "<?php\n\n//anqicms配置文件\nreturn " . var_export($this->config, true) . ";\n";
        $result = file_put_contents($this->configPath, $configString);
        if (!$result) {
            res(1002, "无法写入配置，目录权限不足");
        }
    }
}

/**
 * 数据库操作类
 */
class pdoMysql
{
    private $config = null;
    /** @var PDO */
    public $link = null;
    /** @var PDOStatement|int */
    public $lastqueryid = null;
    public $querycount = 0;
    private $reconnectCount = 0;
    private $sql="";
    public function __construct($config)
    {
        if (!$config['port']) {
            $config['port'] = 3306;//默认端口
        }
        if (!$config['charset']) {
            $config['charset'] = 'utf8';
        }
        $this->config = $config;
        $this->config['dsn'] = 'mysql:host=' . $config['host'] . ';port=' . $config['port'] . ';dbname=' . $config['database'];
        $this->connect();
    }

    private function connect()
    {
        if ($this->reconnectCount > 3) {
            res(1002, '数据库重连失败');
        }
        $this->reconnectCount++;

        try {
            $this->link = new PDO($this->config['dsn'], $this->config['user'], $this->config['password'], array(
                PDO::ATTR_PERSISTENT         => true,
                PDO::ATTR_EMULATE_PREPARES   => false,
                PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
                PDO::MYSQL_ATTR_INIT_COMMAND => "SET NAMES utf8"
            ));
        } catch (Exception $e) {
            res(1002, $e->getMessage());
            $this->link = null;
            return null;
        }
        //重置sql_mode,防止datetime,group by 出错
        $this->link->query("set sql_mode=''");

        return $this->link;
    }

    public function execute($sql)
    {
        if (!is_object($this->link)) {
            $result = $this->connect();
            if (!$result) {
                res(1002, '数据库重连失败');
            }
        }
        $this->lastqueryid = $this->link->exec($sql);
        $errNo = $this->errno();
        if ($errNo == 2003 || $errNo == 2006) {
            $this->connect();
            return $this->execute($sql);
        } elseif ($errNo > 0) {
            res(-1, $this->error($sql));
        }

        $this->querycount++;

        return $this->lastqueryid;
    }

    public function query($sql = null)
    {
        if (!is_object($this->link)) {
            $result = $this->connect();
            if (!$result) {
                res(1002, '数据库重连失败');
            }
        }

        $this->lastqueryid = $this->link->query($sql);
        $errNo = $this->errno();
        if ($errNo == 2003 || $errNo == 2006) {
            $this->connect();
            return $this->query($sql);
        } elseif ($errNo > 0) {
            res(-1, $this->error($sql));
        }

        $this->querycount++;

        return $this->lastqueryid;
    }

    public function select($data, $table, $where = '', $limit = '', $order = '', $group = '', $key = '')
    {
        $where = $where == '' ? '' : ' WHERE ' . $where;
        $order = $order == '' ? '' : ' ORDER BY ' . $order;
        $group = $group == '' ? '' : ' GROUP BY ' . $group;
        $limit = $limit == '' ? '' : ' LIMIT ' . $limit;

        $sql = 'SELECT ' . $data . ' FROM `' . $this->config['database'] . '`.`' . $this->getTable($table) . '`' . $where . $group . $order . $limit;

        $this->query($sql);
        if (!is_object($this->lastqueryid)) {
            return $this->lastqueryid;
        }
        $datalist = $this->lastqueryid->fetchAll();
        if ($key) {
            $datalist_new = array();
            foreach ($datalist as $i => $item) {
                $datalist_new[$item[$key]] = $item;
            }
            $datalist = $datalist_new;
            unset($datalist_new);
        }
        $this->freeResult();

        return $datalist;
    }

    public function getOne($data, $table, $where = '', $order = '', $group = '')
    {
        $where = $where == '' ? '' : ' WHERE ' . $where;
        $order = $order == '' ? '' : ' ORDER BY ' . $order;
        $group = $group == '' ? '' : ' GROUP BY ' . $group;
        $limit = ' LIMIT 1';

        $table = explode(' as ',$table);
        $table = '`.`' . $this->getTable($table[0]) . '`' . ($table[1]?' as '.$table[1]:'');
        $sql = 'SELECT ' . $data . ' FROM `' . $this->config['database'] .$table.  $where . $group . $order . $limit;
        $this->query($sql);
        $res = $this->lastqueryid->fetch();
        $this->freeResult();

        return $res;
    }

    public function getOneCol($data, $table, $where = '', $order = '', $group = '')
    {
        $where = $where == '' ? '' : ' WHERE ' . $where;
        $order = $order == '' ? '' : ' ORDER BY ' . $order;
        $group = $group == '' ? '' : ' GROUP BY ' . $group;
        $limit = ' LIMIT 1';

        $fieldname = str_replace('`', '', $data);
        $sql = 'SELECT ' . $data . ' FROM `' . $this->config['database'] . '`.`' . $this->getTable($table) . '`' . $where . $group . $order . $limit;
        $this->query($sql);
        $res = $this->lastqueryid->fetch();
        $this->freeResult();

        $result = isset($res[$fieldname]) ? $res[$fieldname] : false;

        return $result;
    }

    public function count($where, $table, $group = '')
    {
        $r = $this->getOne("COUNT(*) AS num", $table, $where, '', $group);
        return $r['num'];
    }

    public function fetchAll($res = null)
    {
        $type = PDO::FETCH_ASSOC;
        if ($res) {
            $res_query = $res;
        } else {
            $res_query = $this->lastqueryid;
        }

        return $res_query->fetchAll($type);
    }

    public function getPrimary($table)
    {
        $this->query("SHOW COLUMNS FROM " . $this->getTable($table));
        while ($r = $this->lastqueryid->fetch()) {
            if ($r['Key'] == 'PRI') {
                break;
            }
        }

        return $r['Field'];
    }

    public function getFields($table)
    {
        $fields = array();
        $this->query("SHOW COLUMNS FROM " . $this->getTable($table));
        while ($r = $this->lastqueryid->fetch()) {
            $fields[$r['Field']] = $r['Type'];
        }

        return $fields;
    }

    public function checkFields($table, $array)
    {
        $fields = $this->getFields($table);
        $nofields = array();
        foreach ($array as $v) {
            if (!array_key_exists($v, $fields)) {
                $nofields[] = $v;
            }
        }

        return $nofields;
    }

    public function tableExists($table)
    {
        $tables = $this->listTables();

        return in_array($table, $tables) ? 1 : 0;
    }

    public function listTables()
    {
        $tables = array();
        $this->query("SHOW TABLES");
        while ($r = $this->lastqueryid->fetch()) {
            $tables[] = $r['Tables_in_' . $this->config['database']];
        }

        return $tables;
    }

    public function fieldExists($table, $field)
    {
        $fields = $this->getFields($table);

        return array_key_exists($field, $fields);
    }

    public function getTable($table)
    {
        if (!$this->config['prefix']) {
            return $table;
        }
        if (strpos($table, $this->config['prefix']) === false) {
            return $this->config['prefix'] . $table;
        }

        return $table;
    }

    public function numRows($sql)
    {
        $this->query($sql);

        return $this->lastqueryid->rowCount();
    }

    public function num_fields($sql)
    {
        $this->query($sql);

        return $this->lastqueryid->columnCount();
    }

    public function result($sql, $row)
    {
        $this->query($sql);

        return $this->lastqueryid->fetchColumn($row);
    }

    public function error($msg = null)
    {
        $err = $this->link->errorInfo();
        if($msg){
            $err[] = $msg;
        }
        return $err;
    }

    public function errno()
    {
        return intval($this->link->errorCode());
    }

    public function insertId()
    {
        return $this->link->lastInsertId();
    }

    public function freeResult()
    {
        if (is_object($this->lastqueryid)) {
            $this->lastqueryid = null;
        }
    }

    public function close()
    {
        if (is_object($this->link)) {
            unset($this->link);
        }
    }

    public function halt($message = '', $sql = '')
    {
        res(-1, 'Errno :' . $sql . implode(' ', $this->link->errorInfo()));
    }
}

/**
 * json输出
 * @param      $code
 * @param null $msg
 * @param null $data
 * @param null $extra
 */
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
    $from = $_GET['from'];
    if ($from == 'x') {
        res($code, $msg);
    }

    @header('Content-Type:text/html;charset=UTF-8');
    echo "<div style='padding: 50px;text-align: center;'>$msg</div>";
    die;
}

function nl2p($text)
{
    $text = preg_replace_callback('/(.*)\r?\n\s*/', function($matches){
        if (substr(trim($matches[1]), -1) != ">") {
            return "<p>{$matches[1]}</p>\n";
        } else {
            $str = $matches[0];
        }
        return $str;
    }, $text);
    return $text;
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
