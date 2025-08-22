import os
import pandas as pd
import re
from datetime import datetime
import glob

def extract_organization_name(filename):
    """从文件名中提取组织单位名称"""
    # 文件名格式：组织单位名称-年-月-日--时间戳.xlsx
    match = re.match(r'^(.+?)-(\d{4}-\d{2}-\d{2})--(\d+)\.xlsx$', filename)
    if match:
        return match.group(1)
    return None

def extract_domains_and_ips(domain_column):
    """从域名列中提取域名和IP地址"""
    domains_and_ips = []
    
    for item in domain_column:
        if pd.isna(item) or item == '':
            continue
            
        # 将字符串转换为字符串类型
        item_str = str(item).strip()
        
        # 如果包含换行符，分割成多个项目
        if '\n' in item_str:
            items = item_str.split('\n')
        else:
            items = [item_str]
        
        for single_item in items:
            single_item = single_item.strip()
            if single_item and single_item != 'nan':
                domains_and_ips.append(single_item)
    
    # 数据去重：使用set去重，然后转换为列表并保持顺序
    unique_domains_and_ips = []
    seen_items = set()
    
    for item in domains_and_ips:
        # 标准化处理：去除前后空格，转换为小写进行比较
        normalized_item = item.strip().lower()
        if normalized_item not in seen_items:
            seen_items.add(normalized_item)
            unique_domains_and_ips.append(item)  # 保留原始格式
    
    return unique_domains_and_ips

def show_usage_instructions():
    """显示使用说明"""
    print("=" * 60)
    print("Excel数据提取工具使用说明")
    print("=" * 60)
    print()
    print("功能概述：")
    print("本工具用于从Excel文件中提取特定表单的数据信息，并生成汇总报告。")
    print()
    print("使用步骤：")
    print("1. 在项目根目录下创建 'targets' 文件夹")
    print("2. 将需要处理的Excel文件放入 'targets' 文件夹")
    print("3. 确保Excel文件包含名为 '数据表单' 的工作表")
    print("4. 确保工作表中包含 '信息字段' 列")
    print("5. 运行脚本：python main.py")
    print()
    print("文件命名要求：")
    print("- 文件名格式：机构名称-年-月-日--时间戳.xlsx")
    print("- 示例：示例机构-2025-01-22--1735845402.xlsx")
    print()
    print("输出结果：")
    print("- 结果将保存在 'results' 目录中")
    print("- 输出文件格式：targets-年月日-时间戳.csv")
    print("- CSV文件包含两列：")
    print("  * 第一列：机构名称")
    print("  * 第二列：该机构的所有相关信息（用换行符分隔，用双引号包围）")
    print()
    print("注意事项：")
    print("- Excel文件必须包含指定的工作表名称")
    print("- 工作表中必须包含指定的列名")
    print("- 文件名必须符合指定格式才能正确提取机构名称")
    print("- 脚本会自动处理包含换行符的数据")
    print("=" * 60)

def process_excel_files():
    """处理targets目录中的所有Excel文件"""
    targets_dir = "targets"
    results_dir = "results"
    
    # 检查targets目录是否存在
    if not os.path.exists(targets_dir):
        print("未找到 'targets' 目录")
        print()
        show_usage_instructions()
        return
    
    # 创建results目录（如果不存在）
    if not os.path.exists(results_dir):
        os.makedirs(results_dir)
    
    # 获取所有Excel文件
    excel_files = glob.glob(os.path.join(targets_dir, "*.xlsx"))
    
    if not excel_files:
        print("在 'targets' 目录中没有找到Excel文件")
        print()
        show_usage_instructions()
        return
    
    # 存储所有结果
    all_results = []
    
    for excel_file in excel_files:
        filename = os.path.basename(excel_file)
        print(f"正在处理文件: {filename}")
        
        # 提取组织单位名称
        organization_name = extract_organization_name(filename)
        if not organization_name:
            print(f"无法从文件名 {filename} 中提取组织单位名称")
            continue
        
        try:
            # 读取Excel文件
            excel_data = pd.read_excel(excel_file, sheet_name=None)
            
            # 查找"ICP备案"表单
            icp_sheet = None
            for sheet_name in excel_data.keys():
                if "ICP备案" in sheet_name:
                    icp_sheet = excel_data[sheet_name]
                    break
            
            if icp_sheet is None:
                print(f"在文件 {filename} 中没有找到'ICP备案'表单")
                continue
            
            # 查找"域名"列
            domain_column = None
            for col in icp_sheet.columns:
                if "域名" in str(col):
                    domain_column = icp_sheet[col]
                    break
            
            if domain_column is None:
                print(f"在文件 {filename} 的ICP备案表单中没有找到'域名'列")
                continue
            
            # 提取域名和IP
            domains_and_ips = extract_domains_and_ips(domain_column)
            
            if domains_and_ips:
                # 将结果添加到列表中
                all_results.append({
                    'organization': organization_name,
                    'domains_ips': domains_and_ips
                })
                print(f"从 {organization_name} 提取到 {len(domains_and_ips)} 个域名/IP")
            else:
                print(f"从 {organization_name} 没有提取到任何域名/IP")
                
        except Exception as e:
            print(f"处理文件 {filename} 时出错: {str(e)}")
            continue
    
    # 生成CSV文件
    if all_results:
        # 生成输出文件名
        current_time = datetime.now()
        timestamp = int(current_time.timestamp())
        output_filename = f"targets-{current_time.strftime('%Y%m%d')}-{timestamp}.csv"
        output_path = os.path.join(results_dir, output_filename)
        
        # 写入CSV文件
        with open(output_path, 'w', encoding='utf-8') as f:
            for result in all_results:
                organization = result['organization']
                domains_ips = '\n'.join(result['domains_ips'])
                
                # 写入CSV行，使用逗号分隔符
                # 如果包含换行符（多个项目），则用双引号包围；否则直接输出
                if '\n' in domains_ips:
                    f.write(f'{organization},"{domains_ips}"\n')
                else:
                    f.write(f'{organization},{domains_ips}\n')
        
        print(f"\n处理完成！结果已保存到: {output_path}")
        print(f"共处理了 {len(all_results)} 个组织的数据")
    else:
        print("没有提取到任何数据")

if __name__ == "__main__":
    print("开始处理Excel文件...")
    process_excel_files()
    print("程序执行完毕！")
