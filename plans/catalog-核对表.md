# 归一核对表（BL-POC-009 Phase 1，引擎真实归一逻辑实测）

裁决原则（2026-11-17）：只收录**有 pocs 绑定条目**的漏收写法（绑定命中修复导向）；无绑定条目的写法变体（NC/GRP-U8/TurboCRM/金蝶云星空/红帆等子产品系）与同名异产品（vmware 应用系 vs ESXi/NSX、思科 Nexus vs Sonatype Nexus）**不收**——误归一会扩大绑定误打面。已收项见 gen_catalog.go extraAliases（含注释依据），失效清理见 staleAliases。

下表为收录后基线的剩余漏收段（均为不收裁决：同名异产品/无绑定条目的子产品变体，见裁决原则）。

## 未到达且疑似漏收（需人工裁决）

| 源 | 指纹库写法 | 引擎输出 | 疑似标准名 |
|---|---|---|---|
## 未到达且疑似漏收（需人工裁决）

| 源 | 指纹库写法 | 引擎输出 | 疑似标准名 |
|---|---|---|---|
| fingers | vmware horizon | vmware horizon | — |
| fingers | vmware oneaccess | vmware oneaccess | — |
| fingers | vmware secure file transfer | vmware secure file transfer | — |
| fingers | 心通达OA | 心通达oa | — |
| goby | CISCO-Nexus-Data-Broker | CISCO-Nexus-Data-Broker | — |
| goby | HITACHI-Cosminexus | HITACHI-Cosminexus | — |
| goby | Kong | Kong | — |
| goby | MIS | MIS | — |
| goby | Nexus-3000-Switch | Nexus-3000-Switch | — |
| goby | Nexus-Router | Nexus-Router | — |
| goby | PHP | PHP | — |
| goby | Redis-Commander | Redis-Commander | — |
| goby | Redis-Exporter | Redis-Exporter | — |
| goby | SAS | SAS | — |
| goby | Tomcat-Manager-Application | Tomcat-Manager-Application | — |
| goby | Tomcat-Monitor-uses-WADL | Tomcat-Monitor-uses-WADL | — |
| goby | VMware-Authentication Service | VMware-Authentication Service | — |
| goby | VMware-ESX | VMware-ESX | — |
| goby | VMware-HCX | VMware-HCX | — |
| goby | VMware-Horizon | VMware-Horizon | — |
| goby | VMware-Horizon-DaaS | VMware-Horizon-DaaS | — |
| goby | VMware-Identity-Manager | VMware-Identity-Manager | — |
| goby | VMware-VirtualCenter | VMware-VirtualCenter | — |
| goby | VMware-vCenter | VMware-vCenter | — |
| goby | VMware-vFabric | VMware-vFabric | — |
| goby | VMware-vRealize-Automation-Appliance | VMware-vRealize-Automation-Appliance | — |
| goby | paloalto-VMware | paloalto-VMware | — |
| goby | thin | thin | — |
| goby | vmware-Carbon-Black-EDR | vmware-Carbon-Black-EDR | — |
| goby | vmware-ESXi | vmware-ESXi | — |
| goby | vmware-NSX | vmware-NSX | — |
| goby | vmware-Server-2 | vmware-Server-2 | — |
| goby | vmware-Site-Recovery-Manager | vmware-Site-Recovery-Manager | — |
| goby | vmware-Spring-Batch | vmware-Spring-Batch | — |
| goby | vmware-SpringBoot-Framework | vmware-SpringBoot-Framework | — |
| goby | vmware-Tanzu-Observability | vmware-Tanzu-Observability | — |
| goby | vmware-Workspace-ONE-Access | vmware-Workspace-ONE-Access | — |
| goby | vmware-vRealize | vmware-vRealize | — |
| goby | yongyou-TurboCRM | yongyou-TurboCRM | — |
| goby | yongyou-U8-Cloud | yongyou-U8-Cloud | — |
| goby | 华天动力OA 8000 | 华天动力OA 8000 | — |
| goby | 帆软-FineReport | 帆软-FineReport | — |
| goby | 帆软-FineReport报表平台 | 帆软-FineReport报表平台 | — |
| goby | 心通达OA | 心通达OA | — |
| goby | 用友-NC 4.X版本 | 用友-NC 4.X版本 | — |
| goby | 用友-NC 5.X版本 | 用友-NC 5.X版本 | — |
| goby | 用友-NC 6.X版本 | 用友-NC 6.X版本 | — |
| goby | 用友-NC-Cloud | 用友-NC-Cloud | — |
| goby | 用友GRP-U8-Cloud | 用友GRP-U8-Cloud | — |
| goby | 用友GRP-U8-行政事业内控管理软件 | 用友GRP-U8-行政事业内控管理软件 | — |
| goby | 用友GRP-U8-高校内控管理软件 | 用友GRP-U8-高校内控管理软件 | — |
| goby | 用友NC | 用友NC | — |
| goby | 用友NC-Cloud | 用友NC-Cloud | — |
| goby | 用友TurboCRM | 用友TurboCRM | — |
| goby | 用友U8C | 用友U8C | — |
| goby | 红帆-iOffice M2后台管理系统 | 红帆-iOffice M2后台管理系统 | — |
| goby | 红帆-ioffice | 红帆-ioffice | — |
| goby | 金蝶云星空 | 金蝶云星空 | — |
| goby | 金蝶云星空-BBC分销子系统 | 金蝶云星空-BBC分销子系统 | — |
| goby | 金蝶云星空ERP-管理中心 | 金蝶云星空ERP-管理中心 | — |

## 到达产品册标准名（核对归类正确）

| 源 | 指纹库写法 | 引擎输出 |
|---|---|---|
| fingers | discuz | discuz |
| fingers | jboss | jboss |
| fingers | jboss | jboss |
| fingers | jboss application server 7 | jboss |
| fingers | sonatype nexus repository manager | nexus |
| fingers | thinkphp | thinkphp |
| fingers | tomcat | tomcat |
| fingers | tongda | 通达 oa |
| fingers | zabbix | zabbix |
| fingers | 协众软件-协众OA | 协众oa |
| fingers | 协达oa | 协达 oa |
| fingers | 泛微e-mobile | 泛微 e-mobile |
| fingers | 用友GRP-U8 | 用友 grp-u8 |
| fingers | 用友U8 | 用友 u8 oa |
| fingers | 蓝凌OA-EKP | 蓝凌 oa |
| fingers | 通达oa | 通达 oa |
| fingers | 齐治堡垒机 | qizhi |
| goby | Apache-Tomcat | tomcat |
| goby | BEA-WebLogic-Server | weblogic |
| goby | Coremail-邮件系统 | coremail |
| goby | Discuz | discuz |
| goby | IBM-DB2 database | ibm-db2 database |
| goby | JBoss-5.0 | jboss |
| goby | JBoss-AS | jboss |
| goby | JBoss-EAP | jboss |
| goby | Jboss | Jboss |
| goby | LandRay-OA system | 蓝凌 oa |
| goby | Nexus-Repository-Manager | nexus |
| goby | PHPOA | PHPOA |
| goby | PhpCMS | 泛普软件 建筑工程施工oa |
| goby | Qi Zhi Technology - Fortress | qizhi |
| goby | Redis | redis |
| goby | TDXK-通达OA | 通达 oa |
| goby | ThinkPHP | thinkphp |
| goby | ThinkPHP-YFCMF | thinkphp |
| goby | TopOffice | TopOffice |
| goby | Weblogic_interface_7001 | weblogic |
| goby | ZABBIX-监控系统 | zabbix |
| goby | yongyou-GRP-U8 | 用友 grp-u8 |
| goby | 协众软件-协众OA | 协众oa |
| goby | 泛微-EMobile | 泛微 e-mobile |
| goby | 泛微-EOffice | 泛微 e-office |
| goby | 泛微-云桥e-Bridge | 泛微 云桥e-bridge |
| goby | 泛微-协同办公OA | 泛微 e-cology |
| goby | 用友-U8 | 用友 u8 oa |
| goby | 用友GRP-U8 | 用友 grp-u8 |
| goby | 用友U8 | 用友 u8 oa |
| goby | 蓝凌OA-EKP | 蓝凌 oa |
| goby | 通达OA | 通达 oa |

## 自标准名（输出原名，统计）

共 7645 条，输出原名参与 Contains 匹配，行为不变，不再逐条列出。
