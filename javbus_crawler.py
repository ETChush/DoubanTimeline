#!/usr/bin/env python3
import asyncio
import argparse
import re
import time
from pathlib import Path
from typing import Dict, Any, List, Tuple, Optional

import aiofiles
import aiofiles.os
from lxml import etree
import httpx


class AsyncWebClient:
    """异步Web客户端，用于HTTP请求和文件下载"""
    
    def __init__(self, proxy: Optional[str] = None, timeout: float = 30.0):
        self.proxy = proxy
        self.timeout = timeout
        self.client = httpx.AsyncClient(proxy=proxy, timeout=timeout, verify=False)
    
    async def __aenter__(self):
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.close()
    
    async def close(self):
        await self.client.aclose()
    
    async def get_text(self, url: str, headers: Optional[Dict[str, str]] = None) -> Tuple[Optional[str], str]:
        """获取URL的文本内容"""
        try:
            response = await self.client.get(url, headers=headers)
            response.raise_for_status()
            return response.text, ""
        except Exception as e:
            return None, str(e)
    
    async def download(self, url: str, file_path: Path, headers: Optional[Dict[str, str]] = None) -> bool:
        """下载文件到指定路径"""
        try:
            async with self.client.stream("GET", url, headers=headers) as response:
                response.raise_for_status()
                with open(file_path, "wb") as f:
                    async for chunk in response.aiter_bytes():
                        f.write(chunk)
            return True
        except Exception as e:
            print(f"下载失败: {url} -> {file_path}，错误: {e}")
            return False


class CrawlerData:
    """爬虫数据结构"""
    
    def __init__(self):
        self.title: str = ""
        self.number: str = ""
        self.poster: str = ""
        self.thumb: str = ""
        self.extrafanart: List[str] = []
        self.image_download: bool = False
        self.image_cut: str = "right"
        self.actor: str = ""
        self.actor_photo: Dict[str, str] = {}
        self.release: str = ""
        self.year: str = ""
        self.tag: str = ""
        self.mosaic: str = ""
        self.runtime: str = ""
        self.studio: str = ""
        self.publisher: str = ""
        self.director: str = ""
        self.series: str = ""
        self.source: str = "javbus"
        self.website: str = ""
        self.trailer: str = ""
        self.wanted: str = ""


class Context:
    """爬虫上下文"""
    
    def __init__(self, input: Any):
        self.input = input
        self.debug_info = DebugInfo()
    
    def debug(self, msg: str):
        """打印调试信息"""
        print(f"[DEBUG] {msg}")
        self.debug_info.debug_logs.append(msg)


class DebugInfo:
    """调试信息"""
    
    def __init__(self):
        self.error: Optional[Exception] = None
        self.search_urls: List[str] = []
        self.detail_urls: List[str] = []
        self.debug_logs: List[str] = []
        self.execution_time: float = 0.0


class CrawlerResult:
    """爬虫结果"""
    
    def __init__(self, **kwargs):
        self.title: str = kwargs.get("title", "")
        self.number: str = kwargs.get("number", "")
        self.poster: str = kwargs.get("poster", "")
        self.thumb: str = kwargs.get("thumb", "")
        self.extrafanart: List[str] = kwargs.get("extrafanart", [])
        self.image_download: bool = kwargs.get("image_download", False)
        self.image_cut: str = kwargs.get("image_cut", "right")
        self.actor: str = kwargs.get("actor", "")
        self.actor_photo: Dict[str, str] = kwargs.get("actor_photo", {})
        self.release: str = kwargs.get("release", "")
        self.year: str = kwargs.get("year", "")
        self.tag: str = kwargs.get("tag", "")
        self.mosaic: str = kwargs.get("mosaic", "")
        self.runtime: str = kwargs.get("runtime", "")
        self.studio: str = kwargs.get("studio", "")
        self.publisher: str = kwargs.get("publisher", "")
        self.director: str = kwargs.get("director", "")
        self.series: str = kwargs.get("series", "")
        self.source: str = kwargs.get("source", "javbus")
        self.website: str = kwargs.get("website", "")
        self.trailer: str = kwargs.get("trailer", "")
        self.wanted: str = kwargs.get("wanted", "")


class CrawlerInput:
    """爬虫输入"""
    
    def __init__(self, number: str, output_dir: str, appoint_url: str = "", mosaic: str = ""):
        self.number = number
        self.output_dir = output_dir
        self.appoint_url = appoint_url
        self.mosaic = mosaic


class CrawlerResponse:
    """爬虫响应"""
    
    def __init__(self, data: Optional[CrawlerResult] = None, debug_info: Optional[DebugInfo] = None):
        self.data = data
        self.debug_info = debug_info or DebugInfo()


class JavbusCrawler:
    """javbus爬虫"""
    
    def __init__(self, proxy: Optional[str] = None):
        self.base_url = "https://www.javbus.com"
        self.proxy = proxy
        self.async_client = AsyncWebClient(proxy=proxy)
    
    async def close(self):
        """关闭资源"""
        await self.async_client.close()
    
    def get_title(self, html: etree._Element) -> str:
        """获取标题"""
        result = html.xpath("//h3/text()")
        return result[0].strip() if result else ""
    
    def get_web_number(self, html: etree._Element, number: str) -> str:
        """获取网页上的番号"""
        result = html.xpath('//span[@class="header"][contains(text(), "識別碼:")]/../span[2]/text()')
        return result[0] if result else number
    
    def get_actor(self, html: etree._Element) -> str:
        """获取演员"""
        try:
            result = html.xpath('//div[@class="star-name"]/a/text()')
            return ",".join(result) if result else ""
        except Exception:
            return ""
    
    def get_actor_photo(self, html: etree._Element, url: str) -> Dict[str, str]:
        """获取演员照片"""
        actor = html.xpath('//div[@class="star-name"]/../a/img/@title')
        photo = html.xpath('//div[@class="star-name"]/../a/img/@src')
        data = {}
        if len(actor) == len(photo):
            for i in range(len(actor)):
                data[actor[i]] = url + photo[i] if "http" not in photo[i] else photo[i]
        else:
            for each in actor:
                data[each] = ""
        return data
    
    def get_cover(self, html: etree._Element, url: str) -> str:
        """获取封面链接"""
        # 先尝试获取小封面
        result = html.xpath('//a[@class="bigImage"]/img/@src')
        if result:
            return (url + result[0] if "http" not in result[0] else result[0])
        
        # 如果小封面获取失败，尝试获取大封面
        result = html.xpath('//a[@class="bigImage"]/@href')
        if result:
            return (url + result[0] if "http" not in result[0] else result[0])
        
        return ""
    
    def get_poster_url(self, cover_url: str) -> str:
        """获取海报URL"""
        if "/pics/" in cover_url:
            return cover_url.replace("/cover/", "/thumb/").replace("_b.jpg", ".jpg")
        elif "/imgs/" in cover_url:
            return cover_url.replace("/cover/", "/thumbs/").replace("_b.jpg", ".jpg")
        return ""
    
    def get_release(self, html: etree._Element) -> str:
        """获取发行日期"""
        result = html.xpath('//span[@class="header"][contains(text(), "發行日期:")]/../text()')
        return result[0].strip() if result else ""
    
    def get_year(self, release: str) -> str:
        """从发行日期获取年份"""
        try:
            result = str(re.search(r"\d{4}", release).group())
            return result
        except Exception:
            return release[:4] if release else ""
    
    def get_mosaic(self, html: etree._Element) -> str:
        """获取马赛克类型"""
        select_tab = str(html.xpath('//li[@class="active"]/a/text()'))
        return "有码" if "有码" in select_tab else "无码"
    
    def get_runtime(self, html: etree._Element) -> str:
        """获取时长"""
        result = html.xpath('//span[@class="header"][contains(text(), "長度:")]/../text()')
        if result:
            result = result[0].strip()
            result = re.findall(r"\d+", result)
            return result[0] if result else ""
        return ""
    
    def get_studio(self, html: etree._Element) -> str:
        """获取制作商"""
        result = html.xpath('//a[contains(@href, "/studio/")]/text()')
        return result[0].strip() if result else ""
    
    def get_publisher(self, html: etree._Element, studio: str) -> str:
        """获取发行商"""
        result = html.xpath('//a[contains(@href, "/label/")]/text()')
        return result[0].strip() if result else studio
    
    def get_director(self, html: etree._Element) -> str:
        """获取导演"""
        result = html.xpath('//a[contains(@href, "/director/")]/text()')
        return result[0].strip() if result else ""
    
    def get_series(self, html: etree._Element) -> str:
        """获取系列"""
        result = html.xpath('//a[contains(@href, "/series/")]/text()')
        return result[0].strip() if result else ""
    
    def get_extra_fanart(self, html: etree._Element, url: str) -> List[str]:
        """获取额外图片"""
        result = html.xpath("//div[@id='sample-waterfall']/a/@href")
        if result:
            new_list = []
            for each in result:
                if "http" not in each:
                    each = url + each
                new_list.append(each)
            return new_list
        return []
    
    def get_tag(self, html: etree._Element) -> str:
        """获取标签"""
        result = html.xpath('//span[@class="genre"]/label/a[contains(@href, "/genre/")]/text()')
        return ",".join(result) if result else ""
    
    async def get_real_url(self, number: str, url_type: str, javbus_url: str, headers: Dict[str, str]) -> str:
        """获取详情页链接"""
        if url_type == "us":  # 欧美
            url_search = "https://www.javbus.hair/search/" + number
        elif url_type == "censored":  # 有码
            url_search = javbus_url + "/search/" + number + "&type=&parent=ce"
        else:  # 无码
            url_search = javbus_url + "/uncensored/search/" + number + "&type=0&parent=uc"

        print(f"搜索地址: {url_search}")
        
        # 搜索番号
        html_search, error = await self.async_client.get_text(url_search, headers=headers)
        if html_search is None:
            error_msg = f"网络请求错误: {error}"
            print(error_msg)
            raise Exception(error_msg)
        
        if "lostpasswd" in html_search:
            raise Exception("Cookie 无效！请重新填写 Cookie 或更新节点！")

        html = etree.fromstring(html_search, etree.HTMLParser())
        url_list = html.xpath("//a[@class='movie-box']/@href")
        
        for each in url_list:
            each_url = each.upper().replace("-", "")
            number_1 = "/" + number.upper().replace(".", "").replace("-", "")
            number_2 = number_1 + "_"
            
            if each_url.endswith(number_1) or number_2 in each_url:
                print(f"番号地址: {each}")
                return each
        
        raise Exception("搜索结果: 未匹配到番号！")
    
    async def download_images(self, result: CrawlerResult, output_dir: Path, only_cover: bool = False, headers: Optional[Dict[str, str]] = None) -> int:
        """下载图片"""
        if not result.image_download:
            print("图片下载被禁用")
            return 0
        
        downloaded_count = 0
        
        if only_cover:
            # 只下载封面，优先使用小封面（poster字段），如果小封面不可用再使用大封面（thumb字段）
            cover_url = result.poster if result.poster else result.thumb
            if cover_url:
                thumb_path = output_dir / "cover.jpg"
                try:
                    await aiofiles.os.makedirs(output_dir, exist_ok=True)
                    if await self.async_client.download(cover_url, thumb_path, headers=headers):
                        print(f"封面下载成功: {cover_url} -> {thumb_path}")
                        downloaded_count += 1
                    else:
                        print(f"封面下载失败: {cover_url}")
                except Exception as e:
                    print(f"封面下载异常: {cover_url} - {e}")
        else:
            # 下载海报
            if result.poster:
                poster_path = output_dir / "poster.jpg"
                try:
                    await aiofiles.os.makedirs(output_dir, exist_ok=True)
                    if await self.async_client.download(result.poster, poster_path, headers=headers):
                        print(f"海报下载成功: {result.poster} -> {poster_path}")
                        downloaded_count += 1
                    else:
                        print(f"海报下载失败: {result.poster}")
                except Exception as e:
                    print(f"海报下载异常: {result.poster} - {e}")
            
            # 下载缩略图
            if result.thumb:
                thumb_path = output_dir / "thumb.jpg"
                try:
                    await aiofiles.os.makedirs(output_dir, exist_ok=True)
                    if await self.async_client.download(result.thumb, thumb_path, headers=headers):
                        print(f"缩略图下载成功: {result.thumb} -> {thumb_path}")
                        downloaded_count += 1
                    else:
                        print(f"缩略图下载失败: {result.thumb}")
                except Exception as e:
                    print(f"缩略图下载异常: {result.thumb} - {e}")
            
            # 下载额外图片
            if result.extrafanart:
                extrafanart_dir = output_dir / "extrafanart"
                await aiofiles.os.makedirs(extrafanart_dir, exist_ok=True)
                
                for i, image_url in enumerate(result.extrafanart):
                    extrafanart_path = extrafanart_dir / f"fanart{i+1}.jpg"
                    try:
                        if await self.async_client.download(image_url, extrafanart_path, headers=headers):
                            print(f"额外图片下载成功: {image_url} -> {extrafanart_path}")
                            downloaded_count += 1
                        else:
                            print(f"额外图片下载失败: {image_url}")
                    except Exception as e:
                        print(f"额外图片下载异常: {image_url} - {e}")
        
        return downloaded_count
    
    async def run(self, number: str, output_dir: str, appoint_url: str = "", mosaic: str = "", download_images: bool = False, only_cover: bool = False) -> Optional[CrawlerResult]:
        """执行爬虫"""
        start_time = time.time()
        print(f"\n{'='*50}")
        print(f"开始爬取 javbus - {number}")
        print(f"{'='*50}")
        
        real_url = appoint_url
        javbus_url = self.base_url
        headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
            "Accept-Language": "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7,ja;q=0.6",
            "Cookie": "existmag=all",  # 添加基本Cookie以绕过某些限制
            "Referer": javbus_url
        }
        
        try:
            if not real_url:
                # 欧美去搜索，其他尝试直接拼接地址，没有结果时再搜索
                if "." in number or re.search(r"[-_]\d{2}[-_]\d{2}[-_]\d{2}", number):  # 欧美影片
                    number = number.replace("-", ".").replace("_", ".")
                    real_url = await self.get_real_url(number, "us", javbus_url, headers)
                else:
                    real_url = javbus_url + "/" + number
                    if number.upper().startswith("CWP") or number.upper().startswith("LAF"):
                        temp_number = number.replace("-0", "-")
                        if temp_number[-2] == "-":
                            temp_number = temp_number.replace("-", "-0")
                        real_url = javbus_url + "/" + temp_number

            print(f"番号地址: {real_url}")
            
            htmlcode, error = await self.async_client.get_text(real_url, headers=headers)
            
            # 判断是否需要登录
            if htmlcode is None:
                raise Exception(f"网络请求错误: {error}")
            
            if "lostpasswd" in htmlcode:
                raise Exception("Cookie 无效！请重新填写 Cookie 或更新节点！")

            # 有404时尝试再次搜索
            if "404" in error:
                # 欧美的不再搜索
                if "." in number:
                    raise Exception("未匹配到番号！")
                
                # 无码搜索结果
                elif mosaic == "无码" or mosaic == "無碼":
                    real_url = await self.get_real_url(number, "uncensored", javbus_url, headers)
                # 有码搜索结果
                else:
                    real_url = await self.get_real_url(number, "censored", javbus_url, headers)
                
                htmlcode, error = await self.async_client.get_text(real_url, headers=headers)
                if htmlcode is None:
                    raise Exception("未匹配到番号！")

            # 获取详情页内容
            html_info = etree.fromstring(htmlcode, etree.HTMLParser())
            title = self.get_title(html_info)
            
            if not title:
                raise Exception("数据获取失败: 未获取到title")
            
            number = self.get_web_number(html_info, number)  # 获取番号，用来替换标题里的番号
            title = title.replace(number, "").strip()
            actor = self.get_actor(html_info)  # 获取actor
            actor_photo = self.get_actor_photo(html_info, javbus_url)
            cover_url = self.get_cover(html_info, javbus_url)  # 获取cover
            poster_url = self.get_poster_url(cover_url)
            release = self.get_release(html_info)
            year = self.get_year(release)
            tag = self.get_tag(html_info)
            mosaic = self.get_mosaic(html_info)
            
            image_cut = "center" if mosaic == "无码" else "right"
            image_download = False
            
            if mosaic == "无码":
                if (
                    "_" in number
                    and poster_url
                    or "HEYZO" in number
                    and len(poster_url.replace(javbus_url + "/imgs/thumbs/", "")) == 7
                ):
                    image_download = True
                # 保留小图地址，以便在only_cover模式下使用
            
            runtime = self.get_runtime(html_info)
            studio = self.get_studio(html_info)
            publisher = self.get_publisher(html_info, studio)
            director = self.get_director(html_info)
            series = self.get_series(html_info)
            extrafanart = self.get_extra_fanart(html_info, javbus_url)
            
            if "KMHRS" in number:  # 剧照第一张是高清图
                image_download = True
                if extrafanart:
                    poster_url = extrafanart[0]
            
            # 构建结果
            result = CrawlerResult(
                title=title,
                number=number,
                poster=poster_url,
                thumb=cover_url,
                extrafanart=extrafanart,
                image_download=image_download or download_images,
                image_cut=image_cut,
                actor=actor,
                actor_photo=actor_photo,
                release=release,
                year=year,
                tag=tag,
                mosaic=mosaic,
                runtime=runtime,
                studio=studio,
                publisher=publisher,
                director=director,
                series=series,
                website=real_url
            )
            
            print(f"\n{'='*30}")
            print(f"爬取结果:")
            print(f"{'='*30}")
            print(f"标题: {result.title}")
            print(f"番号: {result.number}")
            print(f"演员: {result.actor}")
            print(f"发行日期: {result.release}")
            print(f"制作商: {result.studio}")
            print(f"发行商: {result.publisher}")
            print(f"导演: {result.director}")
            print(f"系列: {result.series}")
            print(f"标签: {result.tag}")
            print(f"时长: {result.runtime}")
            print(f"马赛克: {result.mosaic}")
            print(f"来源: {result.website}")
            print(f"封面URL: {result.thumb}")
            print(f"海报URL: {result.poster}")
            
            # 下载图片
            if result.image_download or download_images:
                print(f"\n{'='*30}")
                print(f"开始下载图片:")
                print(f"{'='*30}")
                # 为图片下载添加更完整的headers
                image_headers = headers.copy()
                image_headers["Accept"] = "image/webp,image/apng,image/*,*/*;q=0.8"
                image_headers["Referer"] = result.website  # 使用详情页作为Referer
                downloaded_count = await self.download_images(result, Path(output_dir), only_cover, image_headers)
                print(f"\n图片下载完成，共下载 {downloaded_count} 张图片")
            
            print(f"\n{'='*50}")
            print(f"爬取完成，耗时 {round(time.time() - start_time, 2)} 秒")
            print(f"{'='*50}")
            
            return result
            
        except Exception as e:
            print(f"\n错误: {e}")
            print(f"爬取失败，耗时 {round(time.time() - start_time, 2)} 秒")
            return None


async def main():
    """主函数"""
    parser = argparse.ArgumentParser(description='javbus爬虫脚本')
    parser.add_argument('number', help='影片番号')
    parser.add_argument('--output-dir', '-o', default=str(Path(__file__).parent), help='图片输出目录，默认脚本所在目录')
    parser.add_argument('--proxy', '-p', help='代理服务器地址，例如：http://127.0.0.1:7897')
    parser.add_argument('--url', '-u', default='', help='指定详情页URL')
    parser.add_argument('--mosaic', '-m', default='', choices=['有码', '无码', 'censored', 'uncensored'], help='马赛克类型')
    parser.add_argument('--download-images', '-d', action='store_true', help='强制下载图片')
    parser.add_argument('--only-cover', '-c', action='store_true', help='只下载封面图片')
    parser.add_argument('--json', '-j', action='store_true', help='输出 JSON 格式')

    args = parser.parse_args()

    # 确保输出目录存在
    output_dir = Path(args.output_dir)
    output_dir.mkdir(exist_ok=True)

    # 执行爬虫
    async with AsyncWebClient(proxy=args.proxy) as client:
        crawler = JavbusCrawler(proxy=args.proxy)
        result = await crawler.run(
            number=args.number,
            output_dir=str(output_dir),
            appoint_url=args.url,
            mosaic=args.mosaic,
            download_images=args.download_images,
            only_cover=args.only_cover
        )
        await crawler.close()

        # 如果指定了 JSON 输出，打印 JSON 结果
        if args.json and result:
            import json
            import sys

            # 确保标准输出使用 UTF-8 编码
            if sys.platform == 'win32':
                import codecs
                sys.stdout = codecs.getwriter('utf-8')(sys.stdout.buffer, 'strict')

            output_data = {
                "number": result.number,
                "title": result.title,
                "poster": result.poster,
                "thumb": result.thumb,
                "actor": result.actor,
                "release": result.release,
                "year": result.year,
                "tag": result.tag,
                "mosaic": result.mosaic,
                "runtime": result.runtime,
                "studio": result.studio,
                "publisher": result.publisher,
                "director": result.director,
                "series": result.series,
                "website": result.website
            }
            print("\n__JSON_START__")
            print(json.dumps(output_data, ensure_ascii=False))
            print("__JSON_END__")


if __name__ == "__main__":
    asyncio.run(main())
