<?php
/**
 * dedecms 数据转 anqicms 接口文件
 * 仅支持 php 5.3 以上
 * 版权保护，如需使用，请访问 https://www.anqicms.com/。
 * @author anqicms
 * 微信：websafety
 */

use app\api\model\CmsModel;

// 定义为入口文件
define('IS_INDEX', true);

// 入口文件地址绑定
define('URL_BIND', 'anqicms');

// 引入初始化文件
require dirname(__FILE__) . '/core/init.php';

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
define('ANQI_PATH', dirname(__FILE__) . '/');

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

        $this->configPath = ANQI_PATH . 'anqicms.config.php';
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
        $this->db = new CmsModel();
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
            $result = $this->db->table('ay_model')
            ->where('type=2')
            ->select();

            $modules = [];
            foreach($result as $key => $val) {
              $fields = $this->getModuleFields($val->mcode);
              $tablename = $val->urlname;
              if ($tablename == 'list') {
                $tablename = explode(".", $val->contenttpl)[0];
              }
              $modules[$key] = array(
                'id' => intval($val->mcode),
                'table_name' => $tablename,
                'title' => $val->name,
                'is_system' => intval($val->issystem),
                'title_name' => '',
                'status' => intval($val->status),
                'fields' => $fields,
              );
            }

            res(0, '', $modules);
            break;
        case 'category':
            $fields = array(
                'a.*',
                'b.type'
            );
            $join = array(
                'ay_model b',
                'a.mcode=b.mcode',
                'LEFT'
            );
            $tmpCategories = $this->db->table('ay_content_sort a')->field($fields)
                ->where(array('b.type=2','a.status=1'))
                ->join($join)
                ->order('a.pcode,a.sorting,a.id')
                ->select();
            $categories = [];
            foreach($tmpCategories as $key => $val) {
              $categories[$key] = array(
                'id' => intval($val->scode),
                'parent_id' => intval($val->pcode),
                'title' => $val->name,
                'description' => $val->subname,
                'content' => '',
                'status' => intval($val->status),
                'type' => 1,
                'sort' => 0,
                'url_token' => $val->filename,
                'seo_title' => '',
                'keywords' => $val->keywords,
                'module_id' => intval($val->mcode),
              );
            }
              
            res(0, '', $categories);
            break;
        case 'tag':
            res(0, '', null);
            break;
        
          case 'keyword':
            $result = $this->db->table('ay_tags')->select();
            $keywords = [];
            foreach($result as $key => $val) {
                $keywords[$key] = array(
                    "id" => intval($val->id),
                    "title" => $val->name,
                    'weight' => 1,
                    'link' => $val->link,
                  );
            }
            res(0, '', $keywords);
            break;
        case 'archive':
            $lastId = intval($_GET['last_id']);
            $limit = 100;
            $fields = array(
                'a.*',
                'b.mcode',
            );
            $join = array(
                array(
                    'ay_content_sort b',
                    'a.scode=b.scode',
                    'LEFT'
                ),
                array(
                    'ay_model d',
                    'b.mcode=d.mcode',
                    'LEFT'
                ),
            );
            $where = array(
                "a.id > " . $lastId,
                'd.type=2'
            );
            $result = $this->db->table('ay_content a')
            ->field($fields)
            ->where($where)
            ->join($join)
            ->order("id asc")
            ->limit($limit)
            ->decode()
            ->select();

            $archives = [];
            // 增加附加表
            foreach ($result as $key => $val) {
                $flag = '';
                if ($val->istop == 1) {
                    $flag = ',h';
                }
                if ($val->isrecommend == 1) {
                    $flag = ',c';
                }
                $flag = trim($flag,',');
                $icon = $val->ico;
                if (!empty($icon) && strpos($icon, 'http') === false) {
                    $icon = $this->setting['base_url'] + $icon;
                }
                $images = array();
                if (!empty($icon)) {
                    $images = array($icon);
                }
                $archive = array(
                  'id' => intval($val->id),
                  'title' => $val->title,
                  'keywords' => $val->keywords,
                  'description' => $val->description,
                  'category_id' => intval($val->scode),
                  'views' => intval($val->visits),
                  'status' => intval($val->status),
                  'created_time' => strtotime($val->create_time),
                  'updated_time' => strtotime($val->update_time),
                  'images' => $images,
                  'url_token' => $val->filename,
                  'module_id' => intval($val->mcode),
                  'flag' => $flag,
                  'content' => $val->content,
                );

                $addonResult = $this->db->table('ay_content_ext e')
                    ->where("e.contentid = " . $val->id)
                    ->decode()
                    ->find();

                if (!empty($addonResult)) {
                    $archive['extra'] = (array)$addonResult;
                }
                $archives[$key] = $archive;
            }

            res(0, '', $archives);
            break;
        case 'singlepage':
            $lastId = intval($_GET['last_id']);
            $limit = 100;
            $fields = array(
                'a.*',
                'b.mcode',
            );
            $join = array(
                array(
                    'ay_content_sort b',
                    'a.scode=b.scode',
                    'LEFT'
                ),
                array(
                    'ay_model d',
                    'b.mcode=d.mcode',
                    'LEFT'
                ),
            );
            $where = array(
                'd.type=1'
            );
            $result = $this->db->table('ay_content a')
            ->field($fields)
            ->where($where)
            ->join($join)
            ->order("id asc")
            ->limit($limit)
            ->decode()
            ->select();

            $singlepages = [];
            foreach($result as $key => $val) {
                $singlepages[$key] = array(
                  'id' => intval($val->id),
                  'type' => 3,
                'title' => $val->title,
                'keywords' => $val->keywords,
                'description' => $val->description,
                'status' => intval($val->status),
                'created_time' => strtotime($val->create_time),
                'logo' => $val->ico,
                'url_token' => $val->filename,
                'content' => $val->content,
                );
              }

            res(0, '', $singlepages);
            break;
        case 'static':
            // 打包静态文件，包括模板静态文件、上传的文件
            $file = ANQI_PATH.'anqitmp.zip';
            if(!file_exists($file)) {
              $dir = ANQI_PATH.'static';
                $this->create_zip(rtrim(ANQI_PATH, "/"), $dir, $file);
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

    function getModuleFields($mcode) {
        $fields = array();
        
        $result = $this->db->table('ay_extfield')->order('sorting asc,id asc')
            ->where('mcode='.$mcode)
            ->select();
        
        foreach($result as $key => $val) {
          $fields[] = array(
            "name" => $val->description,
            "field_name" => $val->name,
            "required" => false,
            'is_filter' => false,
            'content' => $val->value,
            'type' => $this->formatType($val->type),
          );
        }

        return $fields;
    }
    
    function formatType($type) {
      if($type == 1 || $type == 7) {
        $type = 'text';
      } else if($type == 5 || $type == 10) {
        $type = 'image';
      } else if($type == 6) {
        $type = 'file';
      } else if($type == 2 || $type == 8) {
        $type = 'textarea';
      } else if ($type == 0) {
        $type = 'number';
      } else if ($type == 3) {
        $type = 'radio';
      } else if ($type == 4) {
        $type = 'checkbox';
      } else if ($type == 9) {
        $type = 'select';
      } else {
        $type = 'text';
      }

      return $type;
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

        $this->checkPbootcms();

        $this->writeConfig();
    }

    private function checkPbootcms()
    {
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
